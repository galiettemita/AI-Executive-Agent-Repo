// Phase v0.6.0C — Read-only Google Calendar adapter types.
//
// Closed types. Adding a field here is a deliberate code change with
// founder review — the whole point of v0.6.0C is to prove that Calendar
// content NEVER leaks past the adapter boundary except for the three
// fields below.

/**
 * The fields Brevio reads from each Google Calendar event. Locked to
 * exactly three:
 *   - summary: event title (replaced with "Busy" for private events;
 *     never the raw title for a private event)
 *   - start: ISO 8601 string of the event start time
 *   - end:   ISO 8601 string of the event end time
 *
 * Explicitly excluded (NEVER cross the adapter boundary):
 *   attendees, description, location, attachments, conferenceData,
 *   organizer, creator, htmlLink, recurringEventId, hangoutLink,
 *   anyoneCanAddSelf, transparency, status, source, originalStartTime,
 *   reminders, extendedProperties, gadget, locked, privateCopy.
 *
 * Per [docs/v0.6.0B-oauth-scope-readiness.md §1] decision rows 5 + 7.
 */
export interface CalendarEvent {
  readonly summary: string;
  readonly start: string;
  readonly end: string;
}

/**
 * The full context object surfaced to consumers (the future ranker
 * prompt builder, the preview script, the audit dispatcher).
 *
 * Lists at most `MAX_EVENTS_IN_CONTEXT` events (caller-supplied). Caller
 * may project further; everything in this object is structural and safe
 * to log AFTER the privacy canary on `events[*].summary` confirms the
 * "Busy" mask survived for private events.
 */
export interface CalendarContext {
  /** All in-window events, ordered by start ascending. */
  readonly events: readonly CalendarEvent[];
  /** Length of `events`, surfaced for convenience and structural audit. */
  readonly event_count_in_window: number;
  /**
   * Minutes from "now" to the start time of the soonest upcoming event.
   * `null` when the window contains no events with a start in the future.
   * Negative when the soonest event is already in progress (i.e. its
   * start is in the past but its end is in the future, still within
   * `windowHours`).
   */
  readonly nearest_event_start_offset_minutes: number | null;
  /** The window the adapter actually used to query Google. */
  readonly window_hours_in_force: number;
  /** Indicates whether this result came from the process-local cache. */
  readonly cache_hit: boolean;
}

/**
 * The minimal subset of Google's events.list response shape that Brevio
 * inspects. Every other field on the raw response is structurally
 * discarded at the adapter boundary (see `projectCalendarEvent`).
 *
 * Visibility semantics:
 *   - 'default' or undefined → use the calendar's default visibility
 *     (treated as non-private)
 *   - 'public' → treated as non-private
 *   - 'private' or 'confidential' → masked to "Busy" per
 *     [docs/v0.6.0B-oauth-scope-readiness.md §1] decision row 7
 */
export interface RawGoogleCalendarEvent {
  readonly summary?: unknown;
  readonly start?: { readonly dateTime?: unknown; readonly date?: unknown };
  readonly end?: { readonly dateTime?: unknown; readonly date?: unknown };
  readonly visibility?: unknown;
  // Every other field present on the wire is intentionally unspecified
  // here. The projection function reads ONLY these four keys (summary,
  // start, end, visibility) and the type system prevents accidental
  // access to anything else from this shape.
}
