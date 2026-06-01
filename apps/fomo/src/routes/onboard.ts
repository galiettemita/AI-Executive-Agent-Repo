// Phase v0.5.1 step #4 — /onboard friend-beta route.
//
// Three routes:
//
//   GET /onboard?token=<plaintext>
//     Renders the consent HTML page. Validates the invite token via
//     LOOKUP only — does NOT consume. Refreshing the page is safe.
//     Renders the privacy copy verbatim (docs/privacy-copy-v0.5.md).
//
//   GET /onboard/start?token=<plaintext>
//     Validates the invite token again (still LOOKUP only), builds
//     the OAuth state + nonce row carrying the token_hash +
//     intended_phone_hash + pre-minted user_uuid + PKCE code_verifier,
//     then 302-redirects to Google's authorize URL.
//
//   GET /onboard/callback?code=&state=
//     Verifies the state HMAC, consumes the onboard nonce, exchanges
//     the OAuth code with Google, then runs a Postgres TRANSACTION:
//       (a) create the users row (id=user_uuid, is_founder=false,
//           phone_e164_encrypted from the invite's bound phone,
//           phone_e164_hash matches intended_phone_hash)
//       (b) persist oauth_tokens for the new user_uuid
//       (c) initialize gmail_cursors for the new user_uuid
//       (d) ATOMICALLY consume the invite_token (UPDATE WHERE
//           token_hash=$1 AND consumed_at IS NULL AND expires_at > now())
//     If ANY step fails, the transaction rolls back. The invite token
//     remains unconsumed — the friend can retry. The founder may
//     re-issue if the token expires.
//
// Founder corrections enforced (see [[multitenant-design-principles]]):
//   #5a Plaintext NEVER persisted (only token_hash on disk).
//   #5b NEVER log plaintext token — audit detail uses token_hash_prefix.
//   #5c Consume AFTER successful user creation — atomic UPDATE inside
//       the transaction.
//   #5d NEVER log raw phone — audit detail uses phone_slug.
//   #5e Phone bound to invite — callback verifies the resolved phone
//       hash equals intended_phone_hash.
//
// IMPORTANT — the friend's phone is NOT derived from Google. Google
// OAuth does not verify phone. We use the phone bound to the invite
// token at issue time (founder vouched). The callback writes
// users.phone_e164_encrypted from the invite-bound phone, not from
// the friend's Google profile.

import { randomUUID } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  type OAuthStateConfig,
  buildState,
  deriveCodeChallenge,
  generateNonce,
  generatePKCEVerifier,
  verifyState
} from '../security/oauth/state.js';
import { exchangeCodeForToken, type FetchLike } from '../security/oauth/exchange.js';
import { type ProviderConfig, buildAuthorizeUrl } from '../security/oauth/providers/index.js';
import { type TokenStore } from '../security/oauth/token-store.js';
import { type GmailCursorStore } from '../memory/gmail-cursors.js';
import { GMAIL_READONLY_SCOPE, type GmailClient } from '../adapters/gmail/client.js';
import {
  type InviteTokenStore,
  InvalidInviteTokenError,
  hashToken,
  tokenHashPrefix
} from '../security/invite-tokens.js';
import {
  type PhoneAllowlistStore,
  decryptInviteBoundPhone,
  hashPhone,
  type PhoneHashConfig,
  phoneSlug
} from '../security/phone-allowlist.js';
import { type CryptoConfig } from '../security/token-crypto.js';
import { type AuditStore } from '../core/audit.js';
import { type KillSwitches } from '../core/kill-switches.js';
import { type SendBlueContactRegistrar } from '../adapters/sendblue/client.js';
import { type MemorySignalStore } from '../memory/memory-signals.js';

const ONBOARD_PROVIDER = 'google';
const ONBOARD_SKILL = 'onboard:friend';

/* ---------------------------------------------------------------------- */
/* HttpResponse — matches the shape used by oauth-google + sendblue-inbound */
/* ---------------------------------------------------------------------- */

export interface HttpResponse {
  readonly status: number;
  readonly headers: Record<string, string>;
  readonly body: string;
}

function htmlResponse(status: number, body: string): HttpResponse {
  return Object.freeze({
    status,
    headers: { 'content-type': 'text/html; charset=utf-8', 'cache-control': 'no-store' },
    body
  });
}

function redirectResponse(location: string): HttpResponse {
  return Object.freeze({
    status: 302,
    headers: { location, 'cache-control': 'no-store' },
    body: ''
  });
}

/* ---------------------------------------------------------------------- */
/* Onboard nonce store (in-memory; PG-backed is future work)              */
/* ---------------------------------------------------------------------- */

// One row per in-flight friend onboarding. The friend's user_uuid is
// pre-minted at /onboard/start time so the OAuth callback can finalize
// the user provisioning atomically. The row carries the invite
// token_hash so the callback can consume it after user creation.
export interface OnboardNonceRow {
  readonly nonce: string;
  readonly token_hash: string;
  readonly intended_phone_hash: string;
  readonly user_uuid: string;
  readonly code_verifier: string;
  readonly created_at: number;
  consumed: boolean;
}

export interface OnboardNonceStore {
  put(row: OnboardNonceRow): Promise<void>;
  consume(nonce: string): Promise<OnboardNonceRow | null>;
}

export class InMemoryOnboardNonceStore implements OnboardNonceStore {
  private readonly rows = new Map<string, OnboardNonceRow>();
  async put(row: OnboardNonceRow): Promise<void> {
    this.rows.set(row.nonce, row);
  }
  async consume(nonce: string): Promise<OnboardNonceRow | null> {
    const row = this.rows.get(nonce);
    if (!row || row.consumed) return null;
    row.consumed = true;
    return row;
  }
}

/* ---------------------------------------------------------------------- */
/* Deps                                                                   */
/* ---------------------------------------------------------------------- */

export interface OnboardRouteDeps {
  readonly inviteStore: InviteTokenStore;
  readonly nonceStore: OnboardNonceStore;
  readonly tokenStore: TokenStore;
  readonly cursorStore: GmailCursorStore;
  readonly phoneAllowlist: PhoneAllowlistStore;
  readonly auditStore: AuditStore;
  // Phase v0.5.1 Step 7 — kill switch gates the dispatcher. When
  // killSwitches.friend_beta_enabled is false, the route is
  // effectively unmounted (dispatcher returns null → default 404).
  // The check is in tryHandleOnboardRequest; individual handlers
  // assume the switch was already on at dispatch time.
  readonly killSwitches: KillSwitches;
  readonly stateConfig: OAuthStateConfig;
  readonly providerConfig: ProviderConfig;
  readonly gmailClient: GmailClient;
  readonly fetchImpl?: FetchLike;
  // Phase v0.5.1 Step 4.1 — the callback decrypts the invite's
  // intended_phone_encrypted envelope using the same AAD it was
  // sealed with (intended_phone_hash). Both crypto + phoneHash
  // configs come from the runtime's existing loaders; there is NO
  // injected placeholder producer.
  readonly crypto: CryptoConfig;
  readonly phoneHash: PhoneHashConfig;
  // Phase v0.5.1 Step 8 fix — privacy copy is loaded ONCE at boot
  // (deps.privacyCopy), but the consent HTML is rendered per-request
  // with the URL's token plaintext as the hidden form input. The
  // earlier Step 4 design baked the token at boot, which would make
  // every friend POST /onboard/start with the same (wrong) token.
  readonly privacyCopy: string;
  // Phase v0.5.3 item #1 — after provisionUser + setPhone succeed,
  // call SendBlue's POST /api/v2/contacts so the friend's number is
  // pre-registered on the account. Without this, v0.5.2 surfaced
  // that SendBlue silently drops both inbound webhooks AND outbound
  // sends to the new contact (the "verified-contact" gate).
  //
  // Optional dep: when SendBlue isn't wired (FOMO_SEND_ENABLED=false)
  // OR the founder is running a localhost-only smoke without SendBlue
  // creds, this is null and the callback writes
  // memory_signals.sendblue_contact_status = {registered: false,
  // error_reason: 'send_disabled'}. The outbound worker (only running
  // when send IS enabled) then refuses to dispatch.
  readonly sendBlueContactRegistrar: SendBlueContactRegistrar | null;
  // Phase v0.5.3 item #1 — memory store, for writing the contact-status
  // signal after the registration attempt. Same store the outbound
  // worker reads from.
  readonly memoryStore: MemorySignalStore;
}

/* ---------------------------------------------------------------------- */
/* Helpers                                                                */
/* ---------------------------------------------------------------------- */

function readToken(url: URL): string | null {
  const t = url.searchParams.get('token');
  if (!t) return null;
  if (t.length > 200 || !/^[A-Za-z0-9_-]+$/.test(t)) return null;
  return t;
}

function renderInvalidPage(reason: 'unknown' | 'expired' | 'consumed' | 'missing'): string {
  const message =
    reason === 'expired'
      ? 'This invite link has expired. Ask the founder to send you a new one.'
      : reason === 'consumed'
        ? 'This invite link has already been used. Ask the founder to send you a new one if needed.'
        : reason === 'missing'
          ? 'No invite token in the URL. Use the link the founder sent you.'
          : 'This invite link is not recognized. Ask the founder to confirm the URL.';
  return `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Brevio — invite not valid</title></head>
<body style="font-family:system-ui,sans-serif;max-width:560px;margin:48px auto;padding:0 16px;line-height:1.5">
<h1 style="margin-bottom:8px">Invite link not valid</h1>
<p>${message}</p>
</body></html>`;
}

function renderSuccessPage(phone_slug: string): string {
  // Wording note: v0.5.1 smoke surfaced that the prior copy ("We'll
  // text the iMessage thread...") read like an imminent action just
  // after OAuth. The new copy makes three things explicit:
  //  1. Onboarding is complete — NO text was sent during onboarding.
  //  2. Future SMS is conditional: an email must look important AND
  //     the founder must approve it in Slack first.
  //  3. STOP/START is always available.
  // The phone ending is shown for transparency (the friend knows
  // which number Brevio will reach them at) but framed as "when the
  // conditions are met," not "any moment now."
  return `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Brevio — connected</title></head>
<body style="font-family:system-ui,sans-serif;max-width:560px;margin:48px auto;padding:0 16px;line-height:1.5">
<h1 style="margin-bottom:8px">You're connected to Brevio</h1>
<p><strong>You're all set — no text was sent during onboarding.</strong></p>
<p>From now on, Brevio reads your Gmail in the background. When an email looks genuinely important, the founder reviews it in Slack first. If they approve it, you'll get a short iMessage at the phone ending in <strong>${phone_slug}</strong>.</p>
<p>You can reply <code>STOP</code> to that iMessage thread at any time to disable Brevio. <code>START</code> re-enables it.</p>
<p style="color:#666;font-size:14px;margin-top:24px">No auto-send. Every alert goes through founder review.</p>
</body></html>`;
}

/* ---------------------------------------------------------------------- */
/* GET /onboard — landing page (LOOKUP only, no consume)                  */
/* ---------------------------------------------------------------------- */

export async function handleOnboardLanding(
  url: URL,
  deps: OnboardRouteDeps
): Promise<HttpResponse> {
  const plaintext = readToken(url);
  if (!plaintext) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.invite_invalid',
      target: 'route:/onboard',
      result: 'failure',
      detail: { stage: 'landing', reason: 'missing' }
    });
    return htmlResponse(400, renderInvalidPage('missing'));
  }
  const token_hash = hashToken(plaintext);
  const record = await deps.inviteStore.lookupByHash(token_hash);
  const now = Date.now();
  let reason: 'unknown' | 'expired' | 'consumed' | null = null;
  if (!record) reason = 'unknown';
  else if (record.consumed_at !== null) reason = 'consumed';
  else if (record.expires_at.getTime() <= now) reason = 'expired';

  if (reason) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.invite_invalid',
      target: 'route:/onboard',
      result: 'failure',
      detail: { stage: 'landing', reason, token_hash_prefix: tokenHashPrefix(token_hash) }
    });
    return htmlResponse(404, renderInvalidPage(reason));
  }
  // Valid — render the consent page with the actual URL token as the
  // hidden form input. Per-request rendering (NOT baked at boot) so
  // every friend's form posts back to /onboard/start with THEIR token.
  return htmlResponse(200, buildConsentPageHtml(deps.privacyCopy, plaintext));
}

/* ---------------------------------------------------------------------- */
/* GET /onboard/start — build OAuth state + redirect to Google            */
/* ---------------------------------------------------------------------- */

export async function handleOnboardStart(
  url: URL,
  deps: OnboardRouteDeps,
  now: () => number = () => Date.now()
): Promise<HttpResponse> {
  const plaintext = readToken(url);
  if (!plaintext) return htmlResponse(400, renderInvalidPage('missing'));
  const token_hash = hashToken(plaintext);
  const record = await deps.inviteStore.lookupByHash(token_hash);
  if (!record) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.invite_invalid',
      target: 'route:/onboard/start',
      result: 'failure',
      detail: { stage: 'start', reason: 'unknown', token_hash_prefix: tokenHashPrefix(token_hash) }
    });
    return htmlResponse(404, renderInvalidPage('unknown'));
  }
  if (record.consumed_at !== null) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.invite_invalid',
      target: 'route:/onboard/start',
      result: 'failure',
      detail: { stage: 'start', reason: 'consumed', token_hash_prefix: tokenHashPrefix(token_hash) }
    });
    return htmlResponse(404, renderInvalidPage('consumed'));
  }
  if (record.expires_at.getTime() <= now()) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.invite_invalid',
      target: 'route:/onboard/start',
      result: 'failure',
      detail: { stage: 'start', reason: 'expired', token_hash_prefix: tokenHashPrefix(token_hash) }
    });
    return htmlResponse(404, renderInvalidPage('expired'));
  }

  // Mint a user_uuid + nonce + code_verifier. Store the in-flight
  // context in the onboard nonce store; the state itself carries only
  // the nonce (HMAC-bound to the iat to enforce TTL).
  const user_uuid = randomUUID();
  const nonce = generateNonce();
  const code_verifier = generatePKCEVerifier();
  const code_challenge = deriveCodeChallenge(code_verifier);

  await deps.nonceStore.put({
    nonce,
    token_hash,
    intended_phone_hash: record.intended_phone_hash,
    user_uuid,
    code_verifier,
    created_at: now(),
    consumed: false
  });

  // Reuse the existing OAuth state shape (HMAC-signed claims).
  // user_id holds the pre-minted user_uuid; skill_id discriminates
  // this is an onboard flow (vs. an existing-user reconnect).
  const state = buildState(deps.stateConfig, {
    user_id: user_uuid,
    provider: ONBOARD_PROVIDER,
    skill_id: ONBOARD_SKILL,
    pending_message_id: null,
    iat: now(),
    nonce
  });

  const authorize_url = buildAuthorizeUrl(
    deps.providerConfig,
    [GMAIL_READONLY_SCOPE],
    state,
    code_challenge
  );
  return redirectResponse(authorize_url);
}

/* ---------------------------------------------------------------------- */
/* GET /onboard/callback — exchange code, transactionally provision      */
/* ---------------------------------------------------------------------- */

export interface OnboardCallbackResult {
  readonly ok: boolean;
  readonly status: number;
  readonly headers: Record<string, string>;
  readonly body: string;
}

export async function handleOnboardCallback(
  url: URL,
  deps: OnboardRouteDeps,
  now: () => number = () => Date.now()
): Promise<HttpResponse> {
  const code = url.searchParams.get('code') ?? '';
  const state = url.searchParams.get('state') ?? '';
  if (!code || !state) {
    return htmlResponse(400, renderInvalidPage('unknown'));
  }

  const claims = verifyState(deps.stateConfig, state, now());
  if (!claims || claims.skill_id !== ONBOARD_SKILL || claims.provider !== ONBOARD_PROVIDER) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.invite_invalid',
      target: 'route:/onboard/callback',
      result: 'failure',
      detail: { stage: 'callback', reason: 'state_verify_failed' }
    });
    return htmlResponse(400, renderInvalidPage('unknown'));
  }
  const nonceRow = await deps.nonceStore.consume(claims.nonce);
  if (!nonceRow) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.invite_invalid',
      target: 'route:/onboard/callback',
      result: 'failure',
      detail: { stage: 'callback', reason: 'nonce_consumed_or_unknown' }
    });
    return htmlResponse(400, renderInvalidPage('unknown'));
  }
  // Defense-in-depth: the state's user_id claim must match the
  // nonce row's user_uuid. Both were minted together at /onboard/start.
  if (nonceRow.user_uuid !== claims.user_id) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.invite_invalid',
      target: 'route:/onboard/callback',
      result: 'failure',
      detail: { stage: 'callback', reason: 'state_user_mismatch' }
    });
    return htmlResponse(400, renderInvalidPage('unknown'));
  }

  // Exchange the OAuth code with Google. NEVER log the response —
  // it carries access_token + refresh_token. exchangeCodeForToken
  // throws OAuthError on any non-2xx; we catch and emit a sanitized
  // audit row.
  let exchange: { access_token: string; refresh_token?: string; expires_at?: Date };
  try {
    const fetchImpl = deps.fetchImpl ?? fetch;
    const result = await exchangeCodeForToken(
      {
        config: deps.providerConfig,
        code,
        codeVerifier: nonceRow.code_verifier
      },
      fetchImpl
    );
    exchange = {
      access_token: result.access_token,
      refresh_token: result.refresh_token,
      expires_at: result.expires_in ? new Date(now() + result.expires_in * 1000) : undefined
    };
  } catch {
    await deps.auditStore.write({
      actor_user_id: nonceRow.user_uuid,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.invite_invalid',
      target: 'route:/onboard/callback',
      result: 'failure',
      detail: { stage: 'callback', reason: 'oauth_exchange_failed' }
    });
    return htmlResponse(502, renderInvalidPage('unknown'));
  }

  // Phase v0.5.1 Step 4.1 — recover the invite-bound phone plaintext
  // from the encrypted envelope stored on the invite_tokens row.
  // Look up the FULL record (not just the nonce row, which only
  // carries the hash). This is a read-only LOOKUP; the actual
  // consume happens later, atomically.
  const inviteRecord = await deps.inviteStore.lookupByHash(nonceRow.token_hash);
  if (!inviteRecord || !inviteRecord.intended_phone_encrypted) {
    await deps.auditStore.write({
      actor_user_id: nonceRow.user_uuid,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.phone_mismatch',
      target: 'route:/onboard/callback',
      result: 'failure',
      detail: {
        stage: 'callback',
        reason: 'invite_bound_phone_missing',
        token_hash_prefix: tokenHashPrefix(nonceRow.token_hash)
      }
    });
    return htmlResponse(500, renderInvalidPage('unknown'));
  }

  // Decrypt the envelope. AAD is the intended_phone_hash — any
  // tamper on either the ciphertext or the AAD context causes the
  // AEAD check to fail. We catch + fail-closed here.
  let bound_phone_plaintext: string;
  try {
    bound_phone_plaintext = decryptInviteBoundPhone(
      inviteRecord.intended_phone_encrypted,
      nonceRow.intended_phone_hash,
      deps.crypto
    );
  } catch {
    await deps.auditStore.write({
      actor_user_id: nonceRow.user_uuid,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.phone_mismatch',
      target: 'route:/onboard/callback',
      result: 'failure',
      detail: {
        stage: 'callback',
        reason: 'invite_bound_phone_decrypt_failed',
        token_hash_prefix: tokenHashPrefix(nonceRow.token_hash)
      }
    });
    return htmlResponse(500, renderInvalidPage('unknown'));
  }

  // Defense-in-depth: re-hash the decrypted plaintext with the
  // current phone-hash key and verify it matches the stored
  // intended_phone_hash. A tampered intended_phone_hash column (with
  // matching ciphertext that decrypted under that AAD) would still
  // fail here — the runtime never trusts a single column.
  const recomputed_hash = hashPhone(bound_phone_plaintext, deps.phoneHash);
  if (recomputed_hash !== nonceRow.intended_phone_hash) {
    await deps.auditStore.write({
      actor_user_id: nonceRow.user_uuid,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.phone_mismatch',
      target: 'route:/onboard/callback',
      result: 'failure',
      detail: {
        stage: 'callback',
        reason: 'invite_bound_phone_hash_mismatch',
        token_hash_prefix: tokenHashPrefix(nonceRow.token_hash)
      }
    });
    return htmlResponse(500, renderInvalidPage('unknown'));
  }

  // Provision the friend user. Ordering matters:
  //   1. Fetch Gmail profile FIRST — we need emailAddress to insert
  //      users (NOT NULL email column) + historyId for the cursor row.
  //   2. provisionUser inserts the users row idempotently (browser
  //      refresh on success page = same insert, ON CONFLICT DO NOTHING).
  //   3. setPhone is a pure UPDATE — requires the users row to exist.
  //   4. tokenStore.save + cursorStore.upsert reference user_id.
  //   5. invite consume is LAST so any failure leaves the token
  //      un-consumed and the friend can retry.
  //
  // Failure handling:
  //   * provisionUser is idempotent — no failure path beyond DB-level
  //     errors which bubble.
  //   * setPhone may throw DuplicatePhoneError → another user owns
  //     this hash. We abort with phone_mismatch audit.
  //   * tokenStore.save / cursorStore.upsert may fail with DB error
  //     → we abort; the partially-created state is acceptable because
  //     the next legitimate retry sees existing rows + idempotent
  //     provisionUser + idempotent setPhone (same user_uuid + same
  //     hash = no-op).
  //   * consume throws InvalidInviteTokenError → token expired or
  //     was already consumed by a race. We abort. The partial state
  //     persists but is harmless (no audit_log / alerts / etc. yet).
  //
  // For v0.5.1 we accept this "best-effort, last-step-atomic" shape;
  // a true SQL-level transaction wrapper is future work.
  let profile: { emailAddress: string; historyId: string };
  try {
    profile = await deps.gmailClient.getProfile(exchange.access_token);
    await deps.phoneAllowlist.provisionUser({
      user_id: nonceRow.user_uuid,
      email: profile.emailAddress
    });
    await deps.phoneAllowlist.setPhone(nonceRow.user_uuid, bound_phone_plaintext);
    await deps.tokenStore.save({
      user_id: nonceRow.user_uuid,
      provider: 'google',
      scopes: [GMAIL_READONLY_SCOPE],
      access_token: exchange.access_token,
      refresh_token: exchange.refresh_token ?? undefined,
      expires_at: exchange.expires_at ?? undefined
    });

    // Initialize gmail_cursors so the polling worker starts from
    // current state, not history-id 0 (which would fetch the entire
    // mailbox).
    await deps.cursorStore.upsert({
      user_id: nonceRow.user_uuid,
      history_id: profile.historyId
    });

    // Phase v0.5.3 item #1 — pre-register the friend with SendBlue
    // BEFORE consuming the invite. v0.5.2 surfaced that SendBlue's
    // "verified-contact" gate silently drops both inbound webhooks
    // AND outbound sends for unregistered contacts. We write
    // memory_signals.sendblue_contact_status either way; outbound
    // worker reads it and refuses to dispatch when registered=false.
    //
    // Per founder correction #1: we do NOT roll back OAuth on
    // registration failure (friend's tokens are valuable; the
    // registration is independently retryable). The outbound gate
    // makes the partial state safe.
    await attemptSendBlueContactRegistration({
      deps,
      user_uuid: nonceRow.user_uuid,
      phone_plaintext: bound_phone_plaintext,
      friend_first_name_hint: profile.emailAddress.split('@')[0]?.slice(0, 32) ?? 'Friend',
      now
    });

    // ATOMIC CONSUME — last step. Throws InvalidInviteTokenError on
    // any of {unknown, consumed, expired}. The partial state created
    // above is left as-is (no rollback in v0.5.1; the friend re-tries
    // with a new invite or the same one if it's somehow still valid).
    await deps.inviteStore.consume({
      token_hash: nonceRow.token_hash,
      consumed_user_id: nonceRow.user_uuid,
      now: new Date(now())
    });

    const slug = phoneSlug(bound_phone_plaintext);
    await deps.auditStore.write({
      actor_user_id: nonceRow.user_uuid,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.user_created',
      target: `user:${nonceRow.user_uuid}`,
      result: 'success',
      detail: {
        token_hash_prefix: tokenHashPrefix(nonceRow.token_hash),
        intended_phone_slug: slug,
        gmail_history_id: profile.historyId
      }
    });
    return htmlResponse(200, renderSuccessPage(slug));
  } catch (err) {
    // We deliberately do NOT classify the error body — that could
    // leak. Just emit the audit row with the failure stage.
    await deps.auditStore.write({
      actor_user_id: nonceRow.user_uuid,
      actor_ip: null,
      actor_user_agent: null,
      action: err instanceof InvalidInviteTokenError ? 'fomo.onboard.invite_invalid' : 'fomo.onboard.phone_mismatch',
      target: 'route:/onboard/callback',
      result: 'failure',
      detail: {
        stage: 'callback',
        reason:
          err instanceof InvalidInviteTokenError
            ? err.reason
            : err instanceof Error && err.name === 'DuplicatePhoneError'
              ? 'duplicate_phone'
              : 'provisioning_failed',
        token_hash_prefix: tokenHashPrefix(nonceRow.token_hash)
      }
    });
    return htmlResponse(500, renderInvalidPage('unknown'));
  }
}

/* ---------------------------------------------------------------------- */
/* Dispatcher                                                              */
/* ---------------------------------------------------------------------- */

export async function tryHandleOnboardRequest(
  req: { method?: string; url?: string },
  deps: OnboardRouteDeps
): Promise<HttpResponse | null> {
  const method = req.method ?? 'GET';
  const url = new URL(req.url ?? '/', 'http://localhost');
  if (method !== 'GET') return null;
  const isOnboardPath =
    url.pathname === '/onboard' ||
    url.pathname === '/onboard/start' ||
    url.pathname === '/onboard/callback';
  if (!isOnboardPath) return null;

  // Phase v0.5.1 Step 7 — kill switch. When FOMO_FRIEND_BETA_ENABLED
  // is false (default), the dispatcher returns null so the request
  // falls through to the server's default 404 handler — the route
  // appears unmounted. We DO emit an audit row so operators can see
  // attempts to hit the route while the switch is off (probes,
  // half-completed friend handoffs, etc.).
  //
  // NOTE: this kill switch gates the /onboard SURFACE only. It does
  // NOT gate the friend-safe Slack card branch (Step 5) — that
  // branch is unconditional on alert.user_id !== founderUserId per
  // the multi-tenant design principles.
  if (!deps.killSwitches.friend_beta_enabled) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.onboard.kill_switch_off',
      target: `route:${url.pathname}`,
      result: 'failure',
      detail: { path: url.pathname }
    });
    return null;
  }

  if (url.pathname === '/onboard') return handleOnboardLanding(url, deps);
  if (url.pathname === '/onboard/start') return handleOnboardStart(url, deps);
  if (url.pathname === '/onboard/callback') return handleOnboardCallback(url, deps);
  return null;
}

/* ---------------------------------------------------------------------- */
/* Privacy copy loader                                                    */
/* ---------------------------------------------------------------------- */

// Resolves docs/privacy-copy-v0.5.md by walking upward from the module
// location. Source layout (apps/fomo/src/routes/) is 4 levels up; compiled
// layout (apps/fomo/dist/src/routes/) is 5 levels up. Walking up makes
// the loader robust to either.
function resolvePrivacyCopyPath(): string {
  let dir = path.dirname(fileURLToPath(import.meta.url));
  for (let i = 0; i < 7; i++) {
    const candidate = path.join(dir, 'docs', 'privacy-copy-v0.5.md');
    if (existsSync(candidate)) return candidate;
    const parent = path.dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  throw new Error(
    `privacy-copy-v0.5.md not found by walking up from ${path.dirname(fileURLToPath(import.meta.url))}`
  );
}

// Reads docs/privacy-copy-v0.5.md at startup. The route renders the
// markdown verbatim (no transformation) at the bottom of the consent
// page. Founder reviews + edits the .md file, not the route source.
export async function loadPrivacyCopy(): Promise<string> {
  return readFile(resolvePrivacyCopyPath(), 'utf8');
}

// Sync companion used at boot. The runtime's createFomoRuntime is
// synchronous; loading the privacy copy via readFile (async) would
// require making boot async too. Reading once at boot is fine via
// sync IO.
export function loadPrivacyCopySync(): string {
  return readFileSync(resolvePrivacyCopyPath(), 'utf8');
}

/* ---------------------------------------------------------------------- */
/* Phase v0.5.3 item #1 — SendBlue contact registration helper            */
/* ---------------------------------------------------------------------- */

interface ContactRegistrationContext {
  deps: OnboardRouteDeps;
  user_uuid: string;
  phone_plaintext: string;
  friend_first_name_hint: string;
  now: () => number;
}

// Best-effort SendBlue contact-add. Always writes a memory_signals
// row recording the outcome — the outbound worker uses that signal
// as the OUTGOING gate. Never throws (failure is recorded, not
// raised — per founder correction #1, registration failure must not
// roll back OAuth or block the rest of the callback chain).
export async function attemptSendBlueContactRegistration(
  ctx: ContactRegistrationContext
): Promise<void> {
  const { deps, user_uuid, phone_plaintext, friend_first_name_hint } = ctx;
  const at = new Date(ctx.now()).toISOString();
  const slug = phoneSlug(phone_plaintext);

  // Send not wired (e.g. FOMO_SEND_ENABLED=false during a localhost
  // founder smoke). Record the disabled state; outbound worker won't
  // run anyway, but the signal is honest.
  if (!deps.sendBlueContactRegistrar) {
    await deps.memoryStore.upsert({
      user_id: user_uuid,
      kind: 'sendblue_contact_status',
      scope_key: null,
      detail: { registered: false, error_reason: 'send_disabled', attempted_at: at },
      source: 'founder_set'
    });
    await deps.auditStore.write({
      actor_user_id: user_uuid,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.contact_registration_failed',
      target: `user:${user_uuid}`,
      result: 'failure',
      detail: { from_slug: slug, error_reason: 'send_disabled' }
    });
    return;
  }

  let outcome;
  try {
    outcome = await deps.sendBlueContactRegistrar.registerContact({
      number: phone_plaintext,
      first_name: friend_first_name_hint
    });
  } catch (err) {
    // SendBlueClient.registerContact catches its own network errors
    // and returns a typed outcome. This catches anything truly
    // unexpected (e.g. a typo in our wiring). Still record + don't
    // re-throw, per correction #1.
    const reason = err instanceof Error ? err.message : String(err);
    await deps.memoryStore.upsert({
      user_id: user_uuid,
      kind: 'sendblue_contact_status',
      scope_key: null,
      detail: { registered: false, error_reason: `unexpected: ${reason.slice(0, 80)}`, attempted_at: at },
      source: 'founder_set'
    });
    await deps.auditStore.write({
      actor_user_id: user_uuid,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.contact_registration_failed',
      target: `user:${user_uuid}`,
      result: 'failure',
      detail: { from_slug: slug, error_reason: 'unexpected_throw' }
    });
    return;
  }

  if (outcome.kind === 'registered') {
    await deps.memoryStore.upsert({
      user_id: user_uuid,
      kind: 'sendblue_contact_status',
      scope_key: null,
      detail: { registered: true, registered_at: at },
      source: 'founder_set'
    });
    await deps.auditStore.write({
      actor_user_id: user_uuid,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.contact_registered',
      target: `user:${user_uuid}`,
      result: 'success',
      detail: { from_slug: slug, http_status: outcome.httpStatus, reason: outcome.reason }
    });
    return;
  }

  // outcome.kind === 'failed'
  await deps.memoryStore.upsert({
    user_id: user_uuid,
    kind: 'sendblue_contact_status',
    scope_key: null,
    detail: { registered: false, error_reason: outcome.reason, attempted_at: at },
    source: 'founder_set'
  });
  await deps.auditStore.write({
    actor_user_id: user_uuid,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.sendblue.contact_registration_failed',
    target: `user:${user_uuid}`,
    result: 'failure',
    detail: { from_slug: slug, http_status: outcome.httpStatus, error_reason: outcome.reason }
  });
}

export function buildConsentPageHtml(privacyCopy: string, tokenPlaintext: string): string {
  // Render the privacy copy verbatim (escape only the < > & for safety
  // — the founder writes the markdown and we trust the input).
  const escaped = privacyCopy
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
  const safeToken = tokenPlaintext.replace(/[^A-Za-z0-9_-]/g, '');
  return `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Brevio — connect Gmail</title></head>
<body style="font-family:system-ui,sans-serif;max-width:680px;margin:48px auto;padding:0 16px;line-height:1.5">
<h1>Connect Gmail to Brevio</h1>
<p>The founder invited you to try Brevio. Read the note below, then click <strong>Connect with Google</strong> to grant Gmail read access.</p>
<hr style="margin:24px 0">
<pre style="white-space:pre-wrap;font-family:inherit;font-size:0.95rem">${escaped}</pre>
<hr style="margin:24px 0">
<form action="/onboard/start" method="get">
  <input type="hidden" name="token" value="${safeToken}">
  <button type="submit" style="font-size:1.05rem;padding:10px 18px;cursor:pointer;background:#1a73e8;color:#fff;border:0;border-radius:4px">Connect with Google</button>
</form>
</body></html>`;
}
