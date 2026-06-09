// Phase v0.6.0C — CalendarContextSource.
//
// Implements the ContextSource seam ([apps/fomo/src/ranker/context-sources/])
// for read-only Google Calendar context.
//
// Hard contract (founder-locked):
//   - Adapter-boundary field exclusion: only summary/start/end leave this
//     module. attendees, description, location, attachments, conferenceData,
//     organizer, creator, htmlLink, recurringEventId, etc. NEVER cross the
//     boundary. Enforcement is structural — `projectCalendarEvent` reads
//     only those three keys.
//   - Private events: `visibility ∈ {'private','confidential'}` → summary
//     replaced with literal "Busy"; start/end preserved.
//   - Default production state OFF: when FOMO_CALENDAR_CONTEXT_ENABLED is
//     not "true", `build()` returns null without making a Calendar API
//     call.
//   - Allowlist gate: when global is on AND user_id is not in
//     FOMO_CALENDAR_CONTEXT_USER_ALLOWLIST, `build()` returns null
//     without making a Calendar API call.
//   - Audit: every non-null build writes a single structural audit row
//     (`brevio.context.calendar_built`) with NO raw event content.
//     Failures route through `sanitizeProviderError` (no raw provider
//     text or HTTP body in the audit detail).
//   - Cache: process-local LRU keyed by (user_id, window_hours). TTL is
//     env-configurable (default 60s). Never written to a DB. Never a
//     memory_signal. Survives only the lifetime of the process.

import { type AuditStore } from '../../core/audit.js';
import {
  sanitizeProviderError,
  type SanitizedProviderError
} from '../../core/sanitize-provider-error.js';
import { type ContextSource, type ContextSourceKind } from '../../ranker/context-sources/index.js';
import {
  CalendarApiError,
  CalendarUnauthorizedError,
  type GoogleCalendarClient
} from './client.js';
import {
  type CalendarContext,
  type CalendarEvent,
  type RawGoogleCalendarEvent
} from './types.js';

/* ====================================================================== */
/* Public types                                                            */
/* ====================================================================== */

export interface CalendarContextOpts {
  /**
   * The window to query: events with start in [now, now + windowHours).
   * Mandatory parameter — there is no implicit default at this layer;
   * the caller (or the deps-level `default_window_hours` fallback) must
   * supply it. Permits non-default windows for future use cases (weekly
   * prep at 168h, travel at 336h) per [docs/v0.6.0B §2.3].
   */
  readonly windowHours: number;
}

export interface CalendarContextSourceDeps {
  /** Concrete HTTP client. Injected for tests. */
  readonly client: GoogleCalendarClient;
  /**
   * Resolves an access_token for `user_id`. The caller's OAuth refresh
   * substrate is responsible for refresh + needs_reauth flips; this
   * function returns `null` only when no token row exists at all.
   */
  readonly getAccessToken: (user_id: string) => Promise<string | null>;
  /** Audit dispatcher. Always called once per non-skipped build. */
  readonly auditStore: AuditStore;
  /** Global kill switch (env: FOMO_CALENDAR_CONTEXT_ENABLED). Default false. */
  readonly enabled: boolean;
  /** Per-user allowlist. Strict === comparison. */
  readonly allowlist: readonly string[];
  /** Cache TTL in ms. Default 60_000. */
  readonly cacheTtlMs: number;
  /** Fallback window if caller passes a non-positive value. Default 48. */
  readonly defaultWindowHours: number;
  /** Maximum events to surface in the projection. Default 50. */
  readonly maxEvents?: number;
  /** Clock injection for tests. Default `Date.now`. */
  readonly now?: () => number;
}

/* ====================================================================== */
/* Cache (process-local LRU; not exported)                                 */
/* ====================================================================== */

interface CacheEntry {
  readonly context: CalendarContext;
  readonly expiresAtMs: number;
}

const MAX_CACHE_ENTRIES = 64;

class TtlLruCache {
  private readonly entries = new Map<string, CacheEntry>();

  get(key: string, nowMs: number): CalendarContext | null {
    const hit = this.entries.get(key);
    if (!hit) return null;
    if (hit.expiresAtMs <= nowMs) {
      this.entries.delete(key);
      return null;
    }
    // Refresh LRU position
    this.entries.delete(key);
    this.entries.set(key, hit);
    return hit.context;
  }

  set(key: string, context: CalendarContext, expiresAtMs: number): void {
    if (this.entries.size >= MAX_CACHE_ENTRIES) {
      const oldest = this.entries.keys().next().value;
      if (oldest !== undefined) this.entries.delete(oldest);
    }
    this.entries.set(key, { context, expiresAtMs });
  }

  // Test helper: not exported in the source-level public API; available
  // through the seam only for the test that asserts TTL expiry path.
  clear(): void {
    this.entries.clear();
  }
}

/* ====================================================================== */
/* CalendarContextSource                                                   */
/* ====================================================================== */

export class CalendarContextSource implements ContextSource<CalendarContext, CalendarContextOpts> {
  readonly kind: ContextSourceKind = 'calendar';
  private readonly cache = new TtlLruCache();
  private readonly deps: CalendarContextSourceDeps;

  constructor(deps: CalendarContextSourceDeps) {
    this.deps = deps;
  }

  /**
   * Test-only: clear the process-local cache. Marked `_internal` so
   * non-test callers do not depend on it.
   */
  _internalResetCacheForTests(): void {
    this.cache.clear();
  }

  async build(userId: string, opts: CalendarContextOpts): Promise<CalendarContext | null> {
    // --- Gate 1: global kill switch ---
    if (!this.deps.enabled) {
      return null;
    }
    // --- Gate 2: per-user allowlist (strict ===) ---
    if (!this.deps.allowlist.includes(userId)) {
      return null;
    }

    const windowHours =
      opts.windowHours > 0 ? opts.windowHours : this.deps.defaultWindowHours;
    const nowMs = (this.deps.now ?? Date.now)();
    const cacheKey = `${userId}|${windowHours}`;

    // --- Cache check ---
    const cached = this.cache.get(cacheKey, nowMs);
    if (cached !== null) {
      // Return a copy with cache_hit=true so the audit reflects the
      // cache outcome for this specific call.
      const hitContext: CalendarContext = {
        events: cached.events,
        event_count_in_window: cached.event_count_in_window,
        nearest_event_start_offset_minutes: cached.nearest_event_start_offset_minutes,
        window_hours_in_force: cached.window_hours_in_force,
        cache_hit: true
      };
      await this.writeSuccessAudit(userId, hitContext);
      return hitContext;
    }

    // --- Cache miss → fetch ---
    const accessToken = await this.deps.getAccessToken(userId);
    if (accessToken === null) {
      // No token row → treat as auth_error. We deliberately pass only
      // http_status (not raw_provider_code), so the sanitizer maps to
      // the closed 'auth_error' reason rather than 'provider_error' via
      // the unknown-code passthrough path.
      await this.writeFailureAudit(
        userId,
        windowHours,
        sanitizeProviderError({ http_status: 401 })
      );
      return null;
    }

    const timeMinIso = new Date(nowMs).toISOString();
    const timeMaxIso = new Date(nowMs + windowHours * 60 * 60 * 1000).toISOString();

    let rawEvents: readonly RawGoogleCalendarEvent[];
    try {
      rawEvents = await this.deps.client.listPrimaryEvents(accessToken, {
        timeMin: timeMinIso,
        timeMax: timeMaxIso
      });
    } catch (err) {
      const sanitized = sanitizeCalendarError(err);
      await this.writeFailureAudit(userId, windowHours, sanitized);
      return null;
    }

    // --- Adapter boundary: project at most maxEvents into closed-type
    //     CalendarEvent. summary/start/end ONLY. Everything else is
    //     structurally discarded. ---
    const maxEvents = this.deps.maxEvents ?? 50;
    const projected: CalendarEvent[] = [];
    for (let i = 0; i < rawEvents.length && projected.length < maxEvents; i++) {
      const candidate = rawEvents[i];
      if (candidate === undefined) continue;
      const event = projectCalendarEvent(candidate);
      if (event === null) continue;
      projected.push(event);
    }

    const frozenEvents = Object.freeze(projected);
    const nearest = computeNearestEventOffsetMinutes(frozenEvents, nowMs);

    const context: CalendarContext = Object.freeze({
      events: frozenEvents,
      event_count_in_window: frozenEvents.length,
      nearest_event_start_offset_minutes: nearest,
      window_hours_in_force: windowHours,
      cache_hit: false
    });

    this.cache.set(cacheKey, context, nowMs + this.deps.cacheTtlMs);
    await this.writeSuccessAudit(userId, context);
    return context;
  }

  /* ----- audit writes (structural only; no raw event content) ----- */

  private async writeSuccessAudit(userId: string, context: CalendarContext): Promise<void> {
    await this.deps.auditStore.write({
      actor_user_id: userId,
      actor_ip: null,
      actor_user_agent: null,
      action: 'brevio.context.calendar_built',
      target: `calendar:${userId}`,
      result: 'success',
      detail: {
        // Structural fields only. NEVER summary text, attendees, etc.
        event_count_in_window: context.event_count_in_window,
        nearest_event_start_offset_minutes: context.nearest_event_start_offset_minutes,
        window_hours_in_force: context.window_hours_in_force,
        source_surface: 'email_alert',
        cache_hit: context.cache_hit
      }
    });
  }

  private async writeFailureAudit(
    userId: string,
    windowHours: number,
    sanitized: SanitizedProviderError
  ): Promise<void> {
    await this.deps.auditStore.write({
      actor_user_id: userId,
      actor_ip: null,
      actor_user_agent: null,
      action: 'brevio.context.calendar_built',
      target: `calendar:${userId}`,
      result: 'failure',
      detail: {
        window_hours_in_force: windowHours,
        source_surface: 'email_alert',
        error_code: sanitized.error_code,
        error_reason: sanitized.error_reason
      }
    });
  }
}

/* ====================================================================== */
/* Pure helpers (exported for tests)                                       */
/* ====================================================================== */

/**
 * Project a raw Google Calendar event into the closed `CalendarEvent`
 * shape. Reads ONLY `summary`, `start`, `end`, `visibility` from the
 * input. All other fields are structurally discarded because this
 * function never references them.
 *
 * Returns `null` when the event has no usable start or end timestamp
 * (the only signals v0.6.0C cares about). Defensive: rejects events
 * with all-day `date` (date-only) shapes for now — Brevio v0.6.0C's
 * "time-sensitive" use case is for timed events; all-day events
 * introduce a timezone question that's out of scope.
 */
export function projectCalendarEvent(raw: RawGoogleCalendarEvent): CalendarEvent | null {
  const startDateTime = typeof raw.start?.dateTime === 'string' ? raw.start.dateTime : null;
  const endDateTime = typeof raw.end?.dateTime === 'string' ? raw.end.dateTime : null;
  if (!startDateTime || !endDateTime) {
    return null;
  }
  const visibility = typeof raw.visibility === 'string' ? raw.visibility : '';
  const isPrivate = visibility === 'private' || visibility === 'confidential';
  const rawSummary = typeof raw.summary === 'string' ? raw.summary : '';
  // Private events: replace summary with "Busy" deterministically.
  // start/end preserved.
  const summary = isPrivate ? 'Busy' : rawSummary;
  return Object.freeze({
    summary,
    start: startDateTime,
    end: endDateTime
  });
}

/**
 * Compute minutes-from-now to the start of the soonest upcoming event.
 * Negative when the soonest event is in progress. `null` when there are
 * no events at all in the projected list.
 */
export function computeNearestEventOffsetMinutes(
  events: readonly CalendarEvent[],
  nowMs: number
): number | null {
  if (events.length === 0) return null;
  // events list is already orderBy=startTime from the Google API; first
  // entry is the soonest start. Defensive: if Google ever returns
  // out-of-order, pick the min ourselves.
  let earliestStartMs = Number.POSITIVE_INFINITY;
  for (const ev of events) {
    const startMs = Date.parse(ev.start);
    if (!Number.isFinite(startMs)) continue;
    if (startMs < earliestStartMs) earliestStartMs = startMs;
  }
  if (!Number.isFinite(earliestStartMs)) return null;
  const deltaMs = earliestStartMs - nowMs;
  return Math.round(deltaMs / 60_000);
}

/**
 * Route a thrown CalendarApiError / CalendarUnauthorizedError through
 * the deny-by-default sanitizer ([apps/fomo/src/core/sanitize-provider-error.ts]).
 * Never inspects message content; always passes structural hints only.
 */
export function sanitizeCalendarError(err: unknown): SanitizedProviderError {
  if (err instanceof CalendarUnauthorizedError) {
    return sanitizeProviderError({ http_status: 401 });
  }
  if (err instanceof CalendarApiError) {
    return sanitizeProviderError({
      raw_provider_code: err.providerCode ?? null,
      http_status: err.httpStatus
    });
  }
  if (err instanceof Error && 'code' in err && typeof (err as { code?: unknown }).code === 'string') {
    return sanitizeProviderError({ network_error_code: (err as { code: string }).code });
  }
  return sanitizeProviderError({});
}
