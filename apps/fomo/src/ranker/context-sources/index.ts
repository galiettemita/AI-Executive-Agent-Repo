// Phase v0.6.0C — ContextSource seam.
//
// Small abstraction for "things that produce structured context the ranker
// MAY consume, on a per-(user_id, alert) basis". The seam exists so future
// context sources (calendar, contacts, threading, longer-window memory)
// can plug in alongside PIL without each one inventing its own shape.
//
// HARD INVARIANT for v0.6.0C: this seam does NOT change the live ranker.
// CalendarContextSource builds + audits + caches, but the result is not
// passed to the production rank call site. The live ranker stays bit-
// identical to v0.5.13. v0.6.0E will be the phase that wires Calendar
// context into the prompt — under its own kill switch + allowlist +
// founder taste check.
//
// PIL is intentionally NOT refactored to this seam in v0.6.0C. Two reasons:
//   1. PIL ships through the live ranker path under its own guarded mode
//      ([[v05-12-pass]] + [[v05-13-pass]]). Touching that path here would
//      pull a TIER 1 live-ranker change into a TIER 1 substrate change for
//      no behavior win.
//   2. The seam is generic enough that PIL can be wrapped in a future
//      adapter when there's a real reason to (e.g. unified preflight or
//      shared audit dispatch). Doing it now is premature abstraction.

/**
 * Generic identity of a context source. Used for audit + observability.
 * Add a new value here when introducing a new ContextSource implementation.
 */
export type ContextSourceKind = 'calendar';

/**
 * A ContextSource produces a typed context value for a (user_id, opts)
 * pair. Returning `null` means "no context applies for this user / this
 * call" — either because the source is dormant (kill switch off), the
 * user is not allowlisted, the upstream lookup returned nothing, or the
 * source determined the input did not warrant a call. Callers must treat
 * `null` as the default, not as a failure signal.
 *
 * Implementations are responsible for their own:
 *   - kill switch enforcement (return null when the switch is off)
 *   - allowlist enforcement (return null when user is not in the list)
 *   - audit writes (kind-specific structural audit only; never raw content)
 *   - sanitized error handling (route provider errors through
 *     sanitizeProviderError; never throw raw provider text up)
 *   - ephemeral caching (no DB writes; process-local only)
 */
export interface ContextSource<TContext, TOpts> {
  readonly kind: ContextSourceKind;
  build(userId: string, opts: TOpts): Promise<TContext | null>;
}
