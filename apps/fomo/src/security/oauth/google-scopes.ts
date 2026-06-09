// Phase v0.6.0C — Google OAuth scope-list helper.
//
// The four authorize-URL / token-save call sites in oauth-google.ts and
// onboard.ts previously hard-coded `[GMAIL_READONLY_SCOPE]`. v0.6.0C adds
// `calendar.events.readonly` conditionally based on the kill switch
// FOMO_CALENDAR_CONTEXT_ENABLED.
//
// This helper centralizes the decision so each call site is ONE line of
// change and the conditional is enforced in one place — the test files
// can assert the truth table in one spot instead of four.
//
// HARD INVARIANT: when FOMO_CALENDAR_CONTEXT_ENABLED is not "true", the
// returned scope array is bit-identical to `[GMAIL_READONLY_SCOPE]`.
// Test C17 (truth table) is the load-bearing coverage.

import { GMAIL_READONLY_SCOPE } from '../../adapters/gmail/client.js';
import { CALENDAR_EVENTS_READONLY_SCOPE } from '../../adapters/google-calendar/client.js';

/**
 * Return the Google OAuth scope list this build should request.
 *
 *   calendar_context_enabled=false → [gmail.readonly]      (v0.5.x baseline)
 *   calendar_context_enabled=true  → [gmail.readonly,
 *                                     calendar.events.readonly]
 *
 * The per-user allowlist is consulted at the CalendarContextSource layer,
 * not here — the consent screen is binary (either the scope is requested
 * or it isn't), and users not in the allowlist still see the broader
 * request when global is on. That's intentional: the substrate is opt-in
 * per user at build-time, not consent-time. Founder is aware.
 */
export function googleAuthorizeScopes(
  calendar_context_enabled: boolean
): readonly string[] {
  if (calendar_context_enabled) {
    return Object.freeze([GMAIL_READONLY_SCOPE, CALENDAR_EVENTS_READONLY_SCOPE]);
  }
  return Object.freeze([GMAIL_READONLY_SCOPE]);
}
