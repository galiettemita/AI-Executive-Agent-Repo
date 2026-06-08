// Phase v0.5.15 — Sanitized Provider Error Reasons (Beta Observability Hardening #3).
//
// SINGLE CHOKEPOINT for any provider-error → audit_log.detail data flow.
// Deny-by-default privacy semantics (founder-locked 2026-06-08):
//
//   * Known provider vocabulary maps to short safe reasons from a locked set.
//   * Unknown / raw provider messages NEVER pass through verbatim.
//   * Free-text inspection is NOT performed — we don't grep raw messages for
//     content. The MERE PRESENCE of a non-empty raw message just elevates
//     "unknown" to "provider_error". Nothing from the raw bytes ever lands
//     in error_code or error_reason.
//
// What this sanitizer NEVER stores:
//   - raw email body / snippet / header
//   - raw reply text
//   - sender_email
//   - phone number
//   - OAuth token
//   - API key / webhook secret / Authorization header
//   - URL with secrets in query
//   - stack trace
//   - JSON blob from provider response body
//   - personal names if they appear in provider text
//
// What it DOES store:
//   - error_code: UPPER_SNAKE machine-readable token, ≤64 chars. Either from
//     the locked allowlist OR a well-shaped passthrough of a structured
//     provider code, OR a generic fallback like 'UNKNOWN' / 'NETWORK_ERROR'.
//   - error_reason: One of the locked SanitizedReason enum values. Never
//     raw text.
//
// Callers should populate the hint object from whatever they have:
//   * Structured provider error fields (e.g. SendBlue's extracted
//     error_message='OPTED_OUT')
//   * HTTP status code if the failure was at the HTTP boundary
//   * Network error code (ECONNRESET, ETIMEDOUT, etc) if pre-HTTP
//   * Optional raw_provider_message — its content is NOT inspected, only
//     its presence/absence as a tie-breaker
//
// This module is pure. No I/O. No logger calls. Tests assert the locked
// behavior exhaustively.

/**
 * The closed set of safe error_reason values. NO free-text. NO provider text.
 * Adding a new value here is a deliberate code change with founder review.
 */
export type SanitizedReason =
  | 'auth_error'
  | 'invalid_argument'
  | 'not_found'
  | 'rate_limited'
  | 'recipient_opted_out'
  | 'recipient_not_registered'
  | 'temporary_provider_error'
  | 'provider_error'
  | 'network_error'
  | 'unknown_error';

export interface SanitizedProviderError {
  readonly error_code: string;
  readonly error_reason: SanitizedReason;
}

/**
 * Input hint object. All fields optional. Pass whatever structured signal
 * the caller has. The raw_provider_message is accepted but NEVER inspected
 * for content (see deny-by-default contract above).
 */
export interface ProviderErrorHint {
  /**
   * Structured provider-side error code (e.g. SendBlue's 'OPTED_OUT', Google
   * OAuth 'invalid_grant'). Looked up against the locked allowlist first;
   * if not found but well-shaped (UPPER_SNAKE), passed through as
   * error_code with error_reason='provider_error'. Non-token-shaped values
   * are ignored (fall through to other hints).
   */
  readonly raw_provider_code?: string | null;
  /**
   * Raw provider message. Content is NEVER inspected. Used only to detect
   * "something failed at the provider level" when no other signal exists
   * (raises 'unknown_error' to 'provider_error').
   */
  readonly raw_provider_message?: string | null;
  /**
   * HTTP status code at the failure boundary. Mapped to safe reason per the
   * locked classification table below.
   */
  readonly http_status?: number | null;
  /**
   * Node network-level error code (ECONNRESET, ETIMEDOUT, ENOTFOUND, etc.).
   * Mapped to 'network_error' if recognized.
   */
  readonly network_error_code?: string | null;
}

/* ============================================================== */
/* Locked allowlist of known provider error codes                  */
/* ============================================================== */

// Constants reused so the sanitizer doesn't allocate per call.
const AUTH_ERROR: SanitizedProviderError = Object.freeze({
  error_code: 'AUTH_ERROR',
  error_reason: 'auth_error'
});
const INVALID_ARGUMENT: SanitizedProviderError = Object.freeze({
  error_code: 'INVALID_ARGUMENT',
  error_reason: 'invalid_argument'
});
const NOT_FOUND: SanitizedProviderError = Object.freeze({
  error_code: 'NOT_FOUND',
  error_reason: 'not_found'
});
const RATE_LIMITED: SanitizedProviderError = Object.freeze({
  error_code: 'RATE_LIMITED',
  error_reason: 'rate_limited'
});
const RECIPIENT_OPTED_OUT: SanitizedProviderError = Object.freeze({
  error_code: 'OPTED_OUT',
  error_reason: 'recipient_opted_out'
});
const RECIPIENT_NOT_REGISTERED: SanitizedProviderError = Object.freeze({
  error_code: 'NOT_REGISTERED',
  error_reason: 'recipient_not_registered'
});
const TEMPORARY_PROVIDER_ERROR: SanitizedProviderError = Object.freeze({
  error_code: 'TEMPORARY_PROVIDER_ERROR',
  error_reason: 'temporary_provider_error'
});
const PROVIDER_ERROR: SanitizedProviderError = Object.freeze({
  error_code: 'PROVIDER_ERROR',
  error_reason: 'provider_error'
});
const UNKNOWN_ERROR: SanitizedProviderError = Object.freeze({
  error_code: 'UNKNOWN',
  error_reason: 'unknown_error'
});

/**
 * Locked allowlist of known provider error codes from the providers Brevio
 * currently integrates with (SendBlue, Google OAuth + Gmail, Slack,
 * Postgres). Adding rows here is a code change with founder review.
 *
 * Keys are normalized to UPPER_SNAKE; the sanitizer normalizes the input
 * before lookup. SendBlue's "SpamRule" (mixed-case) is included as 'SPAMRULE'
 * after normalization.
 */
const KNOWN_PROVIDER_CODE_MAP: Readonly<Record<string, SanitizedProviderError>> = Object.freeze({
  // SendBlue (per apps/fomo/src/adapters/sendblue/client.ts:104-160)
  OPTED_OUT: RECIPIENT_OPTED_OUT,
  SPAMRULE: RECIPIENT_OPTED_OUT,
  RATE_LIMITED: RATE_LIMITED,
  NOT_FOUND: NOT_FOUND,
  INVALID_ARGUMENT: INVALID_ARGUMENT,
  UNAUTHORIZED: AUTH_ERROR,
  CONTACT_NOT_REGISTERED: RECIPIENT_NOT_REGISTERED,

  // Google OAuth (RFC 6749 error codes)
  INVALID_GRANT: AUTH_ERROR,
  INVALID_CLIENT: AUTH_ERROR,
  UNAUTHORIZED_CLIENT: AUTH_ERROR,
  INVALID_REQUEST: INVALID_ARGUMENT,
  INVALID_SCOPE: INVALID_ARGUMENT,
  UNSUPPORTED_GRANT_TYPE: INVALID_ARGUMENT,

  // Slack interactivity verify failure tokens (see slack-interactivity.ts)
  INVALID_SIGNATURE: AUTH_ERROR,
  TIMESTAMP_OUT_OF_WINDOW: AUTH_ERROR,
  MISSING_SIGNATURE_HEADERS: AUTH_ERROR,

  // Generic dispatch / internal classifications already used as error_code
  // in audit detail today. Mapped through the sanitizer so future drift
  // fails closed at the chokepoint.
  BACKEND_ERROR: PROVIDER_ERROR,
  KILL_SWITCH_OFF: INVALID_ARGUMENT,
  BODY_NOT_JSON: INVALID_ARGUMENT,
  BODY_NOT_FORM_ENCODED: INVALID_ARGUMENT,
  MISSING_REQUIRED_FIELDS: INVALID_ARGUMENT,
  UNEXPECTED_PAYLOAD_TYPE: INVALID_ARGUMENT,

  // v0.5.14 feedback ack failure tokens (this was the only raw err.message
  // passthrough; mapping replaces it).
  SEND_THROW: TEMPORARY_PROVIDER_ERROR
});

/**
 * Recognized Node network-level error codes. Anything in this set maps to
 * 'network_error' (transient). Unrecognized network codes fall through to
 * the HTTP-status / raw-message classifiers.
 */
const KNOWN_NETWORK_CODES: ReadonlySet<string> = new Set([
  'ECONNRESET',
  'ETIMEDOUT',
  'ECONNREFUSED',
  'ENOTFOUND',
  'EAI_AGAIN',
  'EPIPE',
  'EHOSTUNREACH',
  'ENETUNREACH',
  'ENETDOWN',
  'EPROTO'
]);

const NETWORK_ERROR_REASON: SanitizedReason = 'network_error';

/**
 * Strict token shape that raw_provider_code MUST match BEFORE normalization.
 * Letter-prefixed; only letters, digits, underscore, hyphen; ≤128 chars
 * pre-bounding. Rejects prose, free-text English, special-character payloads.
 *
 * Examples accepted: 'OPTED_OUT', 'invalid_grant', 'SpamRule', 'OAuth-Error',
 *   'ENOTFOUND', 'invalid_signature'.
 * Examples rejected: 'Your message was declined', 'Free text not a token',
 *   '4xx_error' (starts with digit), 'OAuth-Error!' (exclamation), '' (empty).
 */
const PROVIDER_CODE_INPUT_PATTERN = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/;

/* ============================================================== */
/* Pure sanitizer                                                  */
/* ============================================================== */

/**
 * Sanitize a provider-error hint object into a safe (error_code, error_reason)
 * pair suitable for audit_log.detail. Deny-by-default: raw provider text
 * never appears in the output.
 *
 * Decision order (first match wins):
 *   1. raw_provider_code: looked up against the locked allowlist
 *      (case-insensitive after UPPER_SNAKE normalization). Hit → mapped row.
 *      Miss but well-shaped → passed through as error_code with
 *      error_reason='provider_error'. Non-token-shaped → ignored.
 *   2. network_error_code: looked up against the known network-code set.
 *      Hit → error_code=<upper code>, error_reason='network_error'.
 *      Unknown network codes are ignored (fall through).
 *   3. http_status: mapped via the locked status table.
 *   4. raw_provider_message: NEVER inspected for content. If present and
 *      non-empty, returns PROVIDER_ERROR (we know SOMETHING failed at the
 *      provider level). If absent, returns UNKNOWN_ERROR.
 */
export function sanitizeProviderError(hint: ProviderErrorHint): SanitizedProviderError {
  // 1. Provider code allowlist + well-shaped passthrough.
  //
  // Deny-by-default: input is REJECTED unless it ALREADY matches a strict
  // token shape (letter-prefixed, alphanumeric + underscore + hyphen only).
  // This blocks prose-like inputs ("Your message was declined") from being
  // forced into UPPER_SNAKE form and landing in error_code as a pseudo-token.
  // Only after the shape check do we uppercase and normalize hyphens for
  // the lookup.
  if (typeof hint.raw_provider_code === 'string') {
    const trimmed = hint.raw_provider_code.trim();
    if (trimmed.length > 0 && PROVIDER_CODE_INPUT_PATTERN.test(trimmed)) {
      // Normalize: uppercase + map hyphens to underscores. Cap at 64 chars.
      const normalized = trimmed.toUpperCase().replace(/-/g, '_').slice(0, 64);
      const hit = KNOWN_PROVIDER_CODE_MAP[normalized];
      if (hit) return hit;
      // Well-shaped but unknown: token passed through with generic reason.
      return Object.freeze({
        error_code: normalized,
        error_reason: 'provider_error' as const
      });
    }
    // Either empty/whitespace OR not token-shaped (contains spaces, special
    // characters, prose, etc.). Ignored entirely; fall through to other hints.
  }

  // 2. Network-level error code.
  if (typeof hint.network_error_code === 'string') {
    const trimmed = hint.network_error_code.trim();
    if (trimmed.length > 0) {
      const upper = trimmed.toUpperCase();
      if (KNOWN_NETWORK_CODES.has(upper)) {
        return Object.freeze({
          error_code: upper,
          error_reason: NETWORK_ERROR_REASON
        });
      }
      // Unknown network code: ignored; fall through.
    }
  }

  // 3. HTTP status.
  if (typeof hint.http_status === 'number' && Number.isFinite(hint.http_status)) {
    const s = hint.http_status;
    if (s === 401 || s === 403) return AUTH_ERROR;
    if (s === 404) return NOT_FOUND;
    if (s === 408 || s === 429) return RATE_LIMITED;
    if (s === 422) return INVALID_ARGUMENT;
    if (s === 400) return INVALID_ARGUMENT;
    if (s >= 500 && s <= 599) return TEMPORARY_PROVIDER_ERROR;
    if (s >= 400 && s <= 499) return INVALID_ARGUMENT;
    // 1xx/2xx/3xx in an error sanitizer is operator error — fall through to
    // unknown rather than asserting; deny-by-default favors silent fallback.
  }

  // 4. Raw provider message: presence only, content NEVER inspected.
  if (typeof hint.raw_provider_message === 'string' && hint.raw_provider_message.trim().length > 0) {
    return PROVIDER_ERROR;
  }

  // 5. Last resort.
  return UNKNOWN_ERROR;
}

// Re-export the locked constants for callers that want to reference them
// directly (e.g. when bypassing the hint object for already-classified
// internal failures). Tests assert the closed reason set.
export const SANITIZED_REASONS: readonly SanitizedReason[] = Object.freeze([
  'auth_error',
  'invalid_argument',
  'not_found',
  'rate_limited',
  'recipient_opted_out',
  'recipient_not_registered',
  'temporary_provider_error',
  'provider_error',
  'network_error',
  'unknown_error'
]);

// Exposed read-only for test introspection.
export const _KNOWN_PROVIDER_CODE_MAP_FOR_TESTS = KNOWN_PROVIDER_CODE_MAP;
export const _KNOWN_NETWORK_CODES_FOR_TESTS = KNOWN_NETWORK_CODES;
