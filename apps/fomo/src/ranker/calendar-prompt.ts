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
 * Phase v0.6.0E.1b — Bumped when the assembled prompt INCLUDES a Calendar
 * context block. Bump rationale: v0.6.0E.1 real-model dry-run produced
 * MIXED behavioral output (F1/F2/F5 did not surface calendar offsets;
 * F10 confidence collapse from an EMPTY calendar block). v0.6.0E.1b
 * adds an explicit Calendar guidance paragraph to the block; ranker-
 * v0.4.0 (no guidance) is intentionally retained in history so the
 * dry-run can produce v0.4.0 vs v0.4.1 comparisons if needed later.
 * Symmetric to PROMPT_VERSION_WITH_PIL in prompt.ts.
 */
export const PROMPT_VERSION_WITH_CALENDAR = 'ranker-v0.4.1';

/**
 * Phase v0.6.0E.1b — Calendar guidance paragraph.
 *
 * Anchored verbatim from the founder's E.1b prompt behavior requirements:
 *   - Calendar should influence the reason ONLY when directly relevant
 *     to the email.
 *   - Calendar should NOT rescue spam or unknown commercial blasts.
 *   - Empty calendar should be neutral and should not collapse
 *     score/confidence.
 *   - Private/Busy events must not be described creepily; avoid
 *     "you have something on your calendar" phrasing.
 *   - Avoid CTA-ish language ("worth a quick glance") unless the email
 *     itself asks for action.
 *   - Prefer neutral timing language: "this lines up with…", "appears
 *     tied to…", "same-day scheduling" only when supported.
 *
 * This string is constant (not configurable) and is exported for tests.
 */
export const CALENDAR_GUIDANCE_PARAGRAPH =
  'Calendar guidance: use the calendar ONLY when the email directly relates to a listed event (same person, same topic, or approximately the same time). If the email is spam, an unknown commercial blast, or unrelated to any listed event, ignore the calendar entirely — base your decision on the email alone. Empty calendar ("no events") is neutral information, NOT a signal that the email is unimportant. Private events appear as "Busy" — do not speculate about what they are, and do not say "you have something on your calendar" or similar phrasing. When the calendar IS relevant, mention it neutrally ("this lines up with…", "appears tied to…", "same-day scheduling"). Avoid CTA wording like "worth a quick glance" unless the email itself clearly asks for action.';

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
  const lines: string[] = [];
  if (ctx.events.length === 0) {
    lines.push(`Calendar (next ${windowH}h): no events.`);
  } else {
    lines.push(`Calendar (next ${windowH}h):`);
    const nowMs = now();
    for (const ev of ctx.events) {
      const startMs = Date.parse(ev.start);
      const offset = Number.isFinite(startMs)
        ? formatOffset(startMs - nowMs)
        : 'soon';
      lines.push(`  ${offset}: ${ev.summary}`);
    }
  }
  // Phase v0.6.0E.1b: append the guidance paragraph for BOTH event and
  // empty-window cases. The empty case is where the founder explicitly
  // wants the model to treat "no events" as neutral, NOT a downgrade
  // signal (F10 confidence collapse in v0.6.0E.1).
  lines.push(CALENDAR_GUIDANCE_PARAGRAPH);
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
