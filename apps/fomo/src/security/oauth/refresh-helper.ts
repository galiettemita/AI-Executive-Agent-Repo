// Phase v0.5.3 item #2 — OAuth access-token auto-refresh helper.
//
// v0.5.2 surfaced that the polling worker never called the existing
// refreshAccessToken function in exchange.ts. Gmail access_tokens
// expire after 1h; the worker would mark needs_reauth=true on first
// 401 and stop polling that user until founder manually re-OAuthed
// (or ran ops:refresh-oauth as the v0.5.2-era bridge).
//
// This helper is wired into GmailPollDeps.oauthRefresh. It's called
// at the top of every user-loop iteration:
//   - if expires_at is more than `skewSeconds` in the future → still_valid
//   - else if refresh_token exists → call refreshAccessToken
//       - on success → save new access_token, return refreshed
//       - on 4xx (invalid_grant etc.) → mark needs_reauth + audit + return needs_reauth
//       - on network/5xx → return transient_fail (no needs_reauth flip)
//   - else (no refresh_token stored) → mark needs_reauth + audit + return needs_reauth
//
// Per founder correction #2: invalid/revoked refresh tokens MUST set
// needs_reauth=true, audit, and skip the user safely. They must NOT
// keep retrying (would waste API calls + potentially trip rate limits).

import { type AuditStore } from '../../core/audit.js';
import { type TokenStore } from './token-store.js';
import { refreshAccessToken, OAuthError, type FetchLike } from './exchange.js';
import { type ProviderConfig } from './providers/index.js';
import {
  type OAuthRefreshDep,
  type OAuthRefreshOutcome
} from '../../workers/gmail-poll.js';

export interface BuildOAuthRefreshHelperArgs {
  readonly tokenStore: TokenStore;
  readonly auditStore: AuditStore;
  readonly providerConfig: ProviderConfig;
  // Skew before expiry to proactively refresh. Default 60s — Gmail
  // calls in the same cycle stay valid for at least this long after
  // we decide the token is still good.
  readonly skewSeconds?: number;
  // Optional clock injection for tests.
  readonly now?: () => number;
  // Optional fetch override for tests.
  readonly fetchImpl?: FetchLike;
  // Provider name in the token store. Always 'google' in v0.5.x.
  readonly provider?: string;
}

export function buildOAuthRefreshHelper(args: BuildOAuthRefreshHelperArgs): OAuthRefreshDep {
  const skewMs = (args.skewSeconds ?? 60) * 1000;
  const provider = args.provider ?? 'google';
  const now = args.now ?? Date.now;
  const fetchImpl = args.fetchImpl;

  return Object.freeze({
    async refreshIfNeeded(user_id: string): Promise<OAuthRefreshOutcome> {
      const tokens = await args.tokenStore.list(user_id);
      const token = tokens.find((t) => t.provider === provider);
      if (!token) {
        // No token row at all — nothing to refresh. Caller's
        // existing flow handles "skipped_no_token" already.
        return Object.freeze({ kind: 'still_valid' as const });
      }

      // Decide: do we need to refresh?
      const nowMs = now();
      const expiresAtMs = token.expires_at?.getTime() ?? 0;
      const isExpiredOrSoon = expiresAtMs === 0 || expiresAtMs - nowMs < skewMs;

      // If still fresh AND not marked needs_reauth → nothing to do.
      // If still fresh AND marked needs_reauth → don't try to refresh
      // (this means a previous refresh already failed; the operator
      // has to re-OAuth manually). still_valid here would be
      // misleading; return needs_reauth to keep the worker honest.
      if (!isExpiredOrSoon && !token.needs_reauth) {
        return Object.freeze({ kind: 'still_valid' as const });
      }
      if (!isExpiredOrSoon && token.needs_reauth) {
        // Token marked needs_reauth from a prior failed refresh; we
        // don't auto-retry (would burn API calls). Operator must
        // re-OAuth.
        return Object.freeze({
          kind: 'needs_reauth' as const,
          reason: 'previously_marked_needs_reauth'
        });
      }

      // Token is expired or near-expiry. Need a refresh_token.
      const refreshToken = await args.tokenStore.loadRefreshToken(user_id, provider);
      if (!refreshToken) {
        // Expired access_token + no refresh_token = the only path
        // back is a fresh OAuth grant. Mark needs_reauth + audit.
        await args.tokenStore.markNeedsReauth(user_id, provider);
        await args.auditStore.write({
          actor_user_id: user_id,
          actor_ip: null,
          actor_user_agent: null,
          action: 'fomo.oauth.refresh_failed',
          target: `oauth:${provider}`,
          result: 'failure',
          detail: {
            provider,
            reason: 'no_refresh_token_stored'
          }
        });
        return Object.freeze({
          kind: 'needs_reauth' as const,
          reason: 'no_refresh_token_stored'
        });
      }

      // Call Google. Catch OAuthError specifically — that's the
      // 4xx-with-error-body shape that means the refresh_token is
      // revoked / invalid_grant / expired.
      try {
        const result = fetchImpl
          ? await refreshAccessToken({ refreshToken, config: args.providerConfig }, fetchImpl)
          : await refreshAccessToken({ refreshToken, config: args.providerConfig });
        const newRefresh = result.refresh_token ?? refreshToken;
        const expires_at =
          result.expires_in !== undefined ? new Date(now() + result.expires_in * 1000) : undefined;

        await args.tokenStore.save({
          user_id,
          provider,
          scopes: token.scopes,
          access_token: result.access_token,
          refresh_token: newRefresh,
          expires_at
        });

        await args.auditStore.write({
          actor_user_id: user_id,
          actor_ip: null,
          actor_user_agent: null,
          action: 'fomo.oauth.refreshed',
          target: `oauth:${provider}`,
          result: 'success',
          detail: {
            provider,
            expires_at_iso: expires_at?.toISOString() ?? null,
            refresh_token_rotated: result.refresh_token !== undefined
          }
        });

        return Object.freeze({
          kind: 'refreshed' as const,
          new_expires_at: expires_at ?? new Date(now() + 3600 * 1000)
        });
      } catch (err) {
        // OAuthError with httpStatus 4xx → refresh_token is bad.
        // Mark needs_reauth + audit + skip. NEVER auto-retry.
        if (err instanceof OAuthError && err.httpStatus >= 400 && err.httpStatus < 500) {
          await args.tokenStore.markNeedsReauth(user_id, provider);
          // Surface the safe provider error code without echoing the body.
          const reason = err.providerError ?? `http_${err.httpStatus}`;
          await args.auditStore.write({
            actor_user_id: user_id,
            actor_ip: null,
            actor_user_agent: null,
            action: 'fomo.oauth.refresh_failed',
            target: `oauth:${provider}`,
            result: 'failure',
            detail: {
              provider,
              reason,
              http_status: err.httpStatus
            }
          });
          return Object.freeze({
            kind: 'needs_reauth' as const,
            reason
          });
        }
        // Network / 5xx / unknown — transient. Skip THIS cycle but
        // do NOT mark needs_reauth. Next cycle will retry the refresh.
        const reason = err instanceof Error ? err.message : String(err);
        return Object.freeze({
          kind: 'transient_fail' as const,
          reason: `transient: ${reason.slice(0, 80)}`
        });
      }
    }
  });
}
