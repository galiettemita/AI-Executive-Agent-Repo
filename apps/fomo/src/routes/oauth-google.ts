// OAuth go-live routes — Google read-only Gmail connection.
//
// Phase 3B.1. Two endpoints:
//
//   POST /oauth/google/start
//     Auth: session-middleware (founder/admin only in v0.1).
//     Generates PKCE + state + nonce. Stores the nonce row keyed by
//     nonce. Returns the Google authorize URL the caller redirects to.
//
//   GET /oauth/google/callback
//     Auth: NONE (the redirect comes from Google's domain; auth is
//     proven by the HMAC-signed state parameter carrying user_id).
//     Verifies state, consumes the nonce (single-use), exchanges
//     code for tokens via existing oauth-exchange substrate, stores
//     encrypted tokens, then calls Gmail profile to seed the
//     gmail_cursors history_id. Returns JSON with the user_id +
//     email_address + initial history_id.
//
// All handlers are pure functions over their deps. The HTTP adapter at
// the bottom wires them into Node's http.IncomingMessage. The handler
// can be tested without booting an HTTP server.
//
// The handlers do NOT touch the audit log directly — dispatched
// audit.write through the existing dispatch substrate is the canonical
// path. Phase 3B.1 does not wire those audit calls because adding a
// dispatch dependency to the route handler enlarges the failure surface
// without testing benefit; Phase 3B.2 will wire the polling worker and
// at that point the OAuth audit events come along for free.

import { Buffer } from 'node:buffer';
import type http from 'node:http';

import { type AuthFailure, type AuthenticatedContext, authenticate, isAuthFailure } from '../security/session-middleware.js';
import { type SessionRuntimeConfig } from '../security/session.js';
import {
  type FetchLike as ExchangeFetchLike,
  type TokenResult,
  exchangeCodeForToken
} from '../security/oauth/exchange.js';
import {
  type NonceStore,
  type OAuthStateConfig,
  type StateClaims,
  buildState,
  deriveCodeChallenge,
  generateNonce,
  generatePKCEVerifier,
  verifyState
} from '../security/oauth/state.js';
import {
  type ProviderConfig,
  buildAuthorizeUrl
} from '../security/oauth/providers/index.js';
import { type TokenStore } from '../security/oauth/token-store.js';
import { type GmailCursorStore } from '../memory/gmail-cursors.js';
import { type GmailClient, GmailUnauthorizedError, GmailApiError } from '../adapters/gmail/client.js';
import { googleAuthorizeScopes } from '../security/oauth/google-scopes.js';

/* ---------------------------------------------------------------------- */
/* Deps                                                                   */
/* ---------------------------------------------------------------------- */

export interface OAuthGoogleRouteDeps {
  readonly providerConfig: ProviderConfig;
  readonly stateConfig: OAuthStateConfig;
  readonly nonceStore: NonceStore;
  readonly tokenStore: TokenStore;
  readonly gmailCursorStore: GmailCursorStore;
  readonly gmailClient: GmailClient;
  // Used by exchangeCodeForToken. Defaults to global fetch.
  readonly fetchImpl?: ExchangeFetchLike;
  // session-middleware config; only used by the HTTP adapter (the handler
  // functions receive an already-authenticated user_id).
  readonly sessionConfig?: SessionRuntimeConfig;
  // Phase v0.6.0C — when true, the authorize URL includes
  // calendar.events.readonly alongside gmail.readonly. Default false
  // (omitting this dep yields the v0.5.x baseline scope set).
  readonly calendarContextEnabled?: boolean;
}

/* ---------------------------------------------------------------------- */
/* /oauth/google/start                                                    */
/* ---------------------------------------------------------------------- */

export interface OAuthGoogleStartRequest {
  // From session-middleware.
  readonly user_id: string;
}

export interface OAuthGoogleStartResponse {
  readonly authorize_url: string;
  readonly state: string;
  readonly nonce: string;
}

export async function handleOAuthGoogleStart(
  req: OAuthGoogleStartRequest,
  deps: OAuthGoogleRouteDeps,
  now: () => number = () => Date.now()
): Promise<OAuthGoogleStartResponse> {
  const codeVerifier = generatePKCEVerifier();
  const codeChallenge = deriveCodeChallenge(codeVerifier);
  const nonce = generateNonce();
  const claims: StateClaims = {
    user_id: req.user_id,
    provider: 'google',
    skill_id: 'gmail.read',
    pending_message_id: null,
    iat: now(),
    nonce
  };
  const state = buildState(deps.stateConfig, claims);

  await deps.nonceStore.put({
    nonce,
    user_id: req.user_id,
    provider: 'google',
    skill_id: 'gmail.read',
    code_verifier: codeVerifier,
    pending_message_id: null,
    created_at: now(),
    consumed: false
  });

  const authorize_url = buildAuthorizeUrl(
    deps.providerConfig,
    [...googleAuthorizeScopes(deps.calendarContextEnabled ?? false)],
    state,
    codeChallenge
  );

  return Object.freeze({ authorize_url, state, nonce });
}

/* ---------------------------------------------------------------------- */
/* /oauth/google/callback                                                 */
/* ---------------------------------------------------------------------- */

export interface OAuthGoogleCallbackRequest {
  readonly code: string;
  readonly state: string;
}

export interface OAuthGoogleCallbackSuccess {
  readonly ok: true;
  readonly user_id: string;
  readonly provider: 'google';
  readonly email_address: string;
  readonly gmail_history_id: string;
}

export type OAuthGoogleCallbackErrorCode =
  | 'missing_code_or_state'
  | 'invalid_state'
  | 'nonce_consumed_or_unknown'
  | 'state_user_mismatch'
  | 'exchange_failed'
  | 'token_save_failed'
  | 'gmail_profile_failed';

export interface OAuthGoogleCallbackFailure {
  readonly ok: false;
  readonly code: OAuthGoogleCallbackErrorCode;
  readonly reason: string;
}

export type OAuthGoogleCallbackResult = OAuthGoogleCallbackSuccess | OAuthGoogleCallbackFailure;

export async function handleOAuthGoogleCallback(
  req: OAuthGoogleCallbackRequest,
  deps: OAuthGoogleRouteDeps,
  now: () => number = () => Date.now()
): Promise<OAuthGoogleCallbackResult> {
  if (!req.code || !req.state) {
    return Object.freeze({
      ok: false as const,
      code: 'missing_code_or_state' as const,
      reason: 'code and state query parameters are required'
    });
  }

  const claims = verifyState(deps.stateConfig, req.state, now());
  if (!claims) {
    return Object.freeze({
      ok: false as const,
      code: 'invalid_state' as const,
      reason: 'state parameter failed HMAC verification or expired'
    });
  }

  const nonceRow = await deps.nonceStore.consume(claims.nonce);
  if (!nonceRow) {
    return Object.freeze({
      ok: false as const,
      code: 'nonce_consumed_or_unknown' as const,
      reason: 'nonce was already consumed or never issued (replay protection)'
    });
  }

  // Defense-in-depth: confirm the nonce row's user_id matches the state's.
  // Both came from /start; if they disagree, something is wrong.
  if (nonceRow.user_id !== claims.user_id) {
    return Object.freeze({
      ok: false as const,
      code: 'state_user_mismatch' as const,
      reason: 'nonce row user_id does not match state claims user_id'
    });
  }

  let tokenResult: TokenResult;
  try {
    tokenResult = await exchangeCodeForToken(
      {
        config: deps.providerConfig,
        code: req.code,
        codeVerifier: nonceRow.code_verifier
      },
      deps.fetchImpl
    );
  } catch (err) {
    return Object.freeze({
      ok: false as const,
      code: 'exchange_failed' as const,
      reason: err instanceof Error ? err.message : String(err)
    });
  }

  // Persist the encrypted token. The TokenStore handles AES-256-GCM
  // at-rest encryption via its CryptoConfig.
  try {
    await deps.tokenStore.save({
      user_id: claims.user_id,
      provider: 'google',
      scopes:
        tokenResult.scope?.split(' ') ??
        Array.from(googleAuthorizeScopes(deps.calendarContextEnabled ?? false)),
      access_token: tokenResult.access_token,
      refresh_token: tokenResult.refresh_token,
      expires_at:
        typeof tokenResult.expires_in === 'number'
          ? new Date(now() + tokenResult.expires_in * 1000)
          : undefined
    });
  } catch (err) {
    return Object.freeze({
      ok: false as const,
      code: 'token_save_failed' as const,
      reason: err instanceof Error ? err.message : String(err)
    });
  }

  // Seed the Gmail cursor with the current history_id so the Phase 3B.2
  // polling worker has a starting point.
  let profile;
  try {
    profile = await deps.gmailClient.getProfile(tokenResult.access_token);
  } catch (err) {
    if (err instanceof GmailUnauthorizedError || err instanceof GmailApiError) {
      return Object.freeze({
        ok: false as const,
        code: 'gmail_profile_failed' as const,
        reason: err.message
      });
    }
    return Object.freeze({
      ok: false as const,
      code: 'gmail_profile_failed' as const,
      reason: err instanceof Error ? err.message : String(err)
    });
  }

  await deps.gmailCursorStore.upsert({
    user_id: claims.user_id,
    history_id: profile.historyId
  });

  return Object.freeze({
    ok: true as const,
    user_id: claims.user_id,
    provider: 'google' as const,
    email_address: profile.emailAddress,
    gmail_history_id: profile.historyId
  });
}

/* ---------------------------------------------------------------------- */
/* HTTP adapter                                                           */
/* ---------------------------------------------------------------------- */

export interface HttpResponse {
  readonly status: number;
  readonly headers: Readonly<Record<string, string>>;
  readonly body: string;
}

function jsonResponse(status: number, payload: unknown): HttpResponse {
  return Object.freeze({
    status,
    headers: Object.freeze({ 'content-type': 'application/json' }),
    body: JSON.stringify(payload)
  });
}

function authFailureResponse(failure: AuthFailure): HttpResponse {
  return jsonResponse(failure.status, { error: failure.code, message: failure.message });
}

// Reads an entire request body up to a small limit. POST /start currently
// has an empty body but a future variant may carry options; we accept up
// to 8KB to be defensive without enabling DoS via large payloads.
async function readBody(req: http.IncomingMessage, maxBytes = 8192): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    let total = 0;
    req.on('data', (chunk: Buffer) => {
      total += chunk.length;
      if (total > maxBytes) {
        reject(new Error(`request body exceeds ${maxBytes} bytes`));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => resolve(Buffer.concat(chunks)));
    req.on('error', reject);
  });
}

// Returns the response to send, or null when the request did not match
// any OAuth route. The server in index.ts checks for null and falls
// through to the default 404 handler.
export async function tryHandleOAuthGoogleRequest(
  req: http.IncomingMessage,
  deps: OAuthGoogleRouteDeps
): Promise<HttpResponse | null> {
  const method = req.method ?? 'GET';
  const url = new URL(req.url ?? '/', 'http://localhost');
  const pathname = url.pathname;

  if (method === 'POST' && pathname === '/oauth/google/start') {
    if (!deps.sessionConfig) {
      return jsonResponse(500, {
        error: 'session_not_configured',
        message: 'OAuth start requires sessionConfig to be wired'
      });
    }
    const authResult: AuthenticatedContext | AuthFailure = authenticate(req, deps.sessionConfig);
    if (isAuthFailure(authResult)) {
      return authFailureResponse(authResult);
    }
    // POST body is currently unused but consumed so the connection
    // closes cleanly.
    await readBody(req);
    const result = await handleOAuthGoogleStart({ user_id: authResult.user_id }, deps);
    return jsonResponse(200, result);
  }

  if (method === 'GET' && pathname === '/oauth/google/callback') {
    const code = url.searchParams.get('code') ?? '';
    const state = url.searchParams.get('state') ?? '';
    const result = await handleOAuthGoogleCallback({ code, state }, deps);
    if (result.ok) {
      return jsonResponse(200, result);
    }
    // 400 for client/state-side failures; 502 for upstream provider failures.
    const status =
      result.code === 'exchange_failed' || result.code === 'gmail_profile_failed' ? 502 : 400;
    return jsonResponse(status, { error: result.code, message: result.reason });
  }

  return null;
}
