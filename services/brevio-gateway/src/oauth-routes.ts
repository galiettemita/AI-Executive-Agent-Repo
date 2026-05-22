import type http from 'node:http';

import {
  type ConsentCategory,
  getCategoryForSkill,
  getOAuthScopesForSkill,
  getOAuthProviderForSkill
} from '@brevio/shared';

import type { AuditStore } from './audit.js';
import type { AuthenticatedContext } from './auth-middleware.js';
import type { ConsentStore } from './consent-store.js';
import { type FetchLike, OAuthError, exchangeCodeForToken } from './oauth-exchange.js';
import {
  type NonceStore,
  type OAuthStateConfig,
  buildState,
  deriveCodeChallenge,
  generateNonce,
  generatePKCEVerifier,
  verifyState
} from './oauth-state.js';
import {
  type OAuthProviderId,
  isSupportedProvider,
  buildAuthorizeUrl,
  loadProviderConfig
} from './oauth-providers/index.js';
import type { TokenStore } from './token-store.js';

export interface OAuthRouteDeps {
  consentStore: ConsentStore;
  tokenStore: TokenStore;
  auditStore: AuditStore;
  nonceStore: NonceStore;
  stateConfig: OAuthStateConfig;
  fetchImpl?: FetchLike;
}

interface RouteContext {
  auth: AuthenticatedContext;
  ip: string | null;
  userAgent: string | null;
  requestId: string;
  url: URL;
}

function sendJSON(res: http.ServerResponse, status: number, body: Record<string, unknown>): void {
  res.writeHead(status, { 'content-type': 'application/json' });
  res.end(JSON.stringify(body));
}

function sendError(res: http.ServerResponse, status: number, code: string, message: string, requestId: string): void {
  sendJSON(res, status, { error: { code, message, request_id: requestId } });
}

function getAllowedRedirects(): string[] {
  const raw = process.env.BREVIO_OAUTH_POST_REDIRECT_ALLOWED?.trim();
  if (!raw) return ['http://localhost:3333'];
  return raw.split(',').map((s) => s.trim()).filter(Boolean);
}

// Origin-exact match. Prevents the classic startsWith open-redirect:
//   prefix=http://localhost:3333  must NOT match http://localhost:3333.evil.com
//   prefix=https://app.brevio.com must NOT match https://app.brevio.com.attacker.com
// Allows path-extension only when followed by a separator.
function isRedirectAllowed(target: string): boolean {
  let targetUrl: URL;
  try {
    targetUrl = new URL(target);
  } catch {
    return false;
  }
  const allowed = getAllowedRedirects();
  for (const prefix of allowed) {
    let allowedUrl: URL;
    try {
      allowedUrl = new URL(prefix);
    } catch {
      continue;
    }
    // Origin must match exactly (scheme + host + port).
    if (targetUrl.origin !== allowedUrl.origin) continue;
    // Path: target's path must equal allowed path OR be a sub-path
    // (allowed='/foo' matches '/foo' and '/foo/bar' but NOT '/foobar').
    const allowedPath = allowedUrl.pathname.replace(/\/$/, '');
    const targetPath = targetUrl.pathname;
    if (allowedPath === '' || allowedPath === targetPath || targetPath.startsWith(`${allowedPath}/`)) {
      return true;
    }
  }
  return false;
}

export async function handleOAuthStart(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  ctx: RouteContext,
  deps: OAuthRouteDeps
): Promise<void> {
  void req;
  const provider = ctx.url.searchParams.get('provider');
  const skillId = ctx.url.searchParams.get('skill_id');
  const pendingMessageId = ctx.url.searchParams.get('pending_message_id');

  if (!isSupportedProvider(provider)) {
    sendError(res, 400, 'invalid_provider', 'unknown provider', ctx.requestId);
    return;
  }
  if (!skillId || !/^[a-z0-9-]{1,64}$/.test(skillId)) {
    sendError(res, 400, 'invalid_skill_id', 'invalid skill_id', ctx.requestId);
    return;
  }
  const skillProvider = getOAuthProviderForSkill(skillId);
  if (skillProvider !== provider) {
    sendError(res, 400, 'provider_skill_mismatch', `skill ${skillId} does not use ${provider}`, ctx.requestId);
    return;
  }
  const config = loadProviderConfig(provider as OAuthProviderId);
  if (!config) {
    sendError(res, 503, 'provider_not_configured', `${provider} OAuth client is not configured`, ctx.requestId);
    return;
  }

  const verifier = generatePKCEVerifier();
  const challenge = deriveCodeChallenge(verifier);
  const nonce = generateNonce();
  const claims = {
    user_id: ctx.auth.user_id,
    provider,
    skill_id: skillId,
    pending_message_id: pendingMessageId,
    iat: Date.now(),
    nonce
  };
  const state = buildState(deps.stateConfig, claims);

  await deps.nonceStore.put({
    nonce,
    user_id: ctx.auth.user_id,
    provider,
    skill_id: skillId,
    code_verifier: verifier,
    pending_message_id: pendingMessageId,
    created_at: claims.iat,
    consumed: false
  });

  const scopes = getOAuthScopesForSkill(skillId);
  const authorizeUrl = buildAuthorizeUrl(config, scopes, state, challenge);
  res.writeHead(302, { location: authorizeUrl });
  res.end();
}

export async function handleOAuthCallback(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  ctx: RouteContext,
  deps: OAuthRouteDeps
): Promise<void> {
  void req;
  const code = ctx.url.searchParams.get('code');
  const state = ctx.url.searchParams.get('state');
  const errorParam = ctx.url.searchParams.get('error');
  const postRedirectRaw = ctx.url.searchParams.get('post_redirect') ?? getAllowedRedirects()[0];
  const postRedirect = isRedirectAllowed(postRedirectRaw) ? postRedirectRaw : getAllowedRedirects()[0];

  if (errorParam) {
    await deps.auditStore.write({
      actor_user_id: ctx.auth.user_id,
      actor_ip: ctx.ip,
      actor_user_agent: ctx.userAgent,
      action: 'oauth.connect',
      target: null,
      result: 'failure',
      detail: { error: errorParam }
    });
    res.writeHead(302, { location: `${postRedirect}?oauth_error=${encodeURIComponent(errorParam)}` });
    res.end();
    return;
  }

  if (!code || !state) {
    sendError(res, 400, 'oauth_missing_params', 'code and state required', ctx.requestId);
    return;
  }

  const claims = verifyState(deps.stateConfig, state);
  if (!claims) {
    sendError(res, 400, 'oauth_state_invalid', 'state HMAC mismatch or expired', ctx.requestId);
    return;
  }
  if (claims.user_id !== ctx.auth.user_id) {
    await deps.auditStore.write({
      actor_user_id: ctx.auth.user_id,
      actor_ip: ctx.ip,
      actor_user_agent: ctx.userAgent,
      action: 'oauth.connect',
      target: `provider:${claims.provider}`,
      result: 'failure',
      detail: { reason: 'state_user_mismatch', state_user: claims.user_id }
    });
    sendError(res, 403, 'oauth_state_user_mismatch', 'state bound to different user (possible OAuth fixation)', ctx.requestId);
    return;
  }
  const consumed = await deps.nonceStore.consume(claims.nonce);
  if (!consumed) {
    sendError(res, 400, 'oauth_state_replay', 'nonce already consumed or unknown', ctx.requestId);
    return;
  }

  const config = loadProviderConfig(claims.provider as OAuthProviderId);
  if (!config) {
    sendError(res, 503, 'provider_not_configured', `${claims.provider} OAuth client is not configured`, ctx.requestId);
    return;
  }

  let result;
  try {
    result = await exchangeCodeForToken({ config, code, codeVerifier: consumed.code_verifier }, deps.fetchImpl);
  } catch (err) {
    const oauthErr = err instanceof OAuthError ? err : null;
    await deps.auditStore.write({
      actor_user_id: ctx.auth.user_id,
      actor_ip: ctx.ip,
      actor_user_agent: ctx.userAgent,
      action: 'oauth.connect',
      target: `provider:${claims.provider}`,
      result: 'failure',
      detail: {
        provider_error: oauthErr?.providerError,
        http_status: oauthErr?.httpStatus,
        message: err instanceof Error ? err.message : String(err)
      }
    });
    res.writeHead(302, { location: `${postRedirect}?oauth_error=${encodeURIComponent(oauthErr?.providerError ?? 'oauth_failed')}` });
    res.end();
    return;
  }

  const expiresAt = result.expires_in ? new Date(Date.now() + result.expires_in * 1000) : undefined;
  const scopes = result.scope ? result.scope.split(/\s+/).filter(Boolean) : getOAuthScopesForSkill(claims.skill_id);

  await deps.tokenStore.save({
    user_id: ctx.auth.user_id,
    provider: claims.provider,
    scopes,
    access_token: result.access_token,
    refresh_token: result.refresh_token,
    expires_at: expiresAt
  });

  const category: ConsentCategory | undefined = getCategoryForSkill(claims.skill_id);
  if (category) {
    await deps.consentStore.setCategoryState({
      user_id: ctx.auth.user_id,
      category,
      state: 'granted',
      source: 'oauth_callback'
    });
  }

  await deps.auditStore.write({
    actor_user_id: ctx.auth.user_id,
    actor_ip: ctx.ip,
    actor_user_agent: ctx.userAgent,
    action: 'oauth.connect',
    target: `provider:${claims.provider}`,
    result: 'success',
    detail: { skill_id: claims.skill_id, scopes_count: scopes.length }
  });

  const redirectParams = new URLSearchParams({ oauth_status: 'connected', provider: claims.provider });
  if (claims.pending_message_id) redirectParams.set('pending_message_id', claims.pending_message_id);
  res.writeHead(302, { location: `${postRedirect}?${redirectParams.toString()}` });
  res.end();
}

export type { RouteContext as OAuthRouteContext };
