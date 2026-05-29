// Phase 3G.1 item #3 — boot-time needs_reauth visibility.
//
// At boot, walk the SAME active-user set the polling worker iterates
// (cursorStore.listUserIds()) and surface a WARN for every user whose
// `oauth_tokens.needs_reauth=true`. Polling would otherwise silently
// skip them every cycle.
//
// Factored into a standalone helper so the test suite can exercise it
// directly against in-memory stores (the boot wiring in index.ts is
// not unit-tested as such — testing the helper IS the regression).
//
// Real incident captured: 2026-05-28 UTC. The polling worker skipped
// the founder for 18+ hours because needs_reauth was true. Discovered
// only via a manual psql query.

import { type GmailCursorStore } from '../memory/gmail-cursors.js';
import { type TokenStore } from '../security/oauth/token-store.js';

export interface NeedsReauthFinding {
  readonly user_id: string;
  readonly provider: 'google';
}

export interface NeedsReauthBootCheckDeps {
  readonly cursorStore: Pick<GmailCursorStore, 'listUserIds'>;
  readonly tokenStore: Pick<TokenStore, 'list'>;
}

/**
 * Returns one finding per user (from the polling worker's active-user
 * set) whose Google token row has needs_reauth=true. Read-only; the
 * caller decides how to surface (WARN log line, Slack ping, etc.).
 *
 * Empty array when no findings. Does NOT throw on store errors —
 * callers can wrap; the helper itself is defensive about empty
 * stores returning no users.
 */
export async function findUsersNeedingReauth(
  deps: NeedsReauthBootCheckDeps
): Promise<readonly NeedsReauthFinding[]> {
  const findings: NeedsReauthFinding[] = [];
  const userIds = await deps.cursorStore.listUserIds();
  for (const user_id of userIds) {
    const tokens = await deps.tokenStore.list(user_id);
    const google = tokens.find((t) => t.provider === 'google');
    if (google && google.needs_reauth) {
      findings.push({ user_id, provider: 'google' });
    }
  }
  return Object.freeze(findings);
}
