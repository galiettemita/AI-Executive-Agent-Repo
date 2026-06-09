// Phase v0.6.0D — Calendar context prompt block.
//
// Pure function. Builds the structured block the ranker prompt receives
// AFTER the PIL block and BEFORE the email body, when the production
// rank call site supplies a non-null `calendar_context`.
//
// HARD INVARIANT (carry-forward from v0.6.0C):
//   - The block sees ONLY summary/start/end via the `CalendarContext`
//     shape — `projectCalendarEvent` already discarded attendees,
//     description, location, attachments, conferenceData, organizer,
//     creator, htmlLink, recurringEventId at the adapter boundary.
//   - Private events arrive with summary='Busy' (deterministic mask
//     applied at the adapter). This module preserves that as-is —
//     never reaches around for the raw private title (it cannot;
//     CalendarContext doesn't carry it).
//
// HARD INVARIANT (v0.6.0D):
//   - The production rank call site continues to pass
//     `calendar_context: null` UNCONDITIONALLY. This block runs only in
//     the offline FIXTURE EXPECTATION HARNESS at
//     `apps/fomo/src/eval/calendar-shadow.eval.ts`.
//   - The harness is NOT a behavioral shadow eval. It proves prompt
//     assembly, placement, privacy, and cross-user isolation. It does
//     NOT prove a real model will use Calendar context correctly. That
//     is a v0.6.0E pre-requisite (run the real ranker on the same
//     fixture prompts; founder reviews the live output).
//   - v0.6.0E is the phase that wires non-null calendar_context into
//     the live ranker, under its own kill switch + allowlist + founder
//     taste check.
//
// Format (founder-locked in v0.6.0D scope, lifted from the v0.6.0C
// preview script so the founder taste-checks the same shape the model
// would eventually see):
//
//   Calendar (next 48h):
//     in 4h: 1:1 with Galiette
//     in 9h: Busy
//     in 29h: Board meeting
//
// Empty window:
//
//   Calendar (next 48h): no events.
//
// Null context:
//
//   (empty string — block not appended)

import { type CalendarContext } from '../adapters/google-calendar/types.js';

/**
 * Phase v0.6.0D — Bumped when the assembled prompt INCLUDES a Calendar
 * context block. Symmetric to PROMPT_VERSION_WITH_PIL in prompt.ts.
 * The two-call shadow eval emits BOTH calls with their respective
 * versions for side-by-side founder taste check.
 */
export const PROMPT_VERSION_WITH_CALENDAR = 'ranker-v0.4.0';

/**
 * Build the Calendar context block for the ranker prompt.
 *
 * Returns the empty string when `ctx` is null so callers can safely
 * concatenate without conditional logic. When `ctx` is non-null:
 *
 *   - header line: `Calendar (next ${window_hours_in_force}h):`
 *   - one event line per event in `ctx.events`:
 *       `  in <Xh|<60m>m>: <summary>`
 *     (offset is rounded to minutes when < 60 min away, otherwise to
 *     whole hours; `Busy` already substituted upstream for private)
 *   - empty `ctx.events` → header line becomes
 *       `Calendar (next ${window_hours_in_force}h): no events.`
 *
 * The function deliberately performs NO field exclusion of its own —
 * the adapter boundary (`projectCalendarEvent`) is the load-bearing
 * gate. This function only formats; it cannot leak a field the input
 * type does not expose.
 *
 * `now` is a clock injection for tests + the eval (offsets are
 * computed relative to it). Production callers in v0.6.0E will pass
 * Date.now wrapped; here it is a parameter to keep this function pure.
 */
export function buildCalendarContextBlock(
  ctx: CalendarContext | null,
  now: () => number = Date.now
): string {
  if (ctx === null) return '';
  const windowH = ctx.window_hours_in_force;
  if (ctx.events.length === 0) {
    return `Calendar (next ${windowH}h): no events.`;
  }
  const nowMs = now();
  const lines: string[] = [`Calendar (next ${windowH}h):`];
  for (const ev of ctx.events) {
    const startMs = Date.parse(ev.start);
    const offset = Number.isFinite(startMs)
      ? formatOffset(startMs - nowMs)
      : 'soon';
    lines.push(`  ${offset}: ${ev.summary}`);
  }
  return lines.join('\n');
}

/**
 * Format a millisecond delta as a short, founder-readable offset.
 *   - in-progress (delta <= 0): "now"
 *   - < 60 minutes ahead: "in NNm"
 *   - 60 minutes or more: "in NNh" (whole hours) with optional "NNm" tail
 *     omitted for terseness past the 24h mark
 *
 * Pure helper, exported for unit tests. Adjusting this format here is
 * a deliberate code change with founder review — the eval and the
 * v0.6.0E live prompt use the same formatter, so changes propagate
 * straight to the user-visible signal.
 */
export function formatOffset(deltaMs: number): string {
  if (deltaMs <= 0) return 'now';
  const totalMinutes = Math.round(deltaMs / 60_000);
  if (totalMinutes < 60) return `in ${totalMinutes}m`;
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes - hours * 60;
  if (hours >= 24) {
    // Past 24h, drop the minutes tail; the model only needs the day-scale signal.
    return `in ${hours}h`;
  }
  if (minutes === 0) return `in ${hours}h`;
  return `in ${hours}h ${minutes}m`;
}
