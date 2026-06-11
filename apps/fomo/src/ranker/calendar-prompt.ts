// Phase v0.6.0E.1c — Relevance-gated Calendar prompt block.
//
// PRODUCT PRINCIPLE (founder-locked 2026-06-11):
//   The whole point of Calendar is NOT "the model sees more text." The
//   point is that Brevio becomes more context-aware in a useful,
//   human-feeling way. Calendar should help Brevio understand WHY an
//   email matters in the user's real-life context. It should not add
//   noise, destabilize scoring, increase cost without visible benefit,
//   or create creepy phrasing.
//
//   v0.6.0E.1b (the "long guidance paragraph" iteration) demonstrated
//   that dumping calendar metadata into the prompt and asking the
//   model to figure it out:
//     - F1/F5 did not surface calendar specificity
//     - F6/F10 collapsed score on empty calendars
//     - +25% input-token cost for marginal lift
//
//   v0.6.0E.1c flips the design: the SYSTEM curates context BEFORE the
//   model sees it. The block is omitted entirely unless we have a
//   direct relevance signal between the email and a specific event.
//
// HARD CONTRACT (founder-locked v0.6.0E.1c scope):
//   1. ctx === null or zero events → return ''.  Empty calendar should
//      be neutral by ABSENCE, not by instruction.
//   2. ctx has events but none are directly relevant to the email →
//      return ''.  Do not dump the calendar list.
//   3. The only event-renderer path is exactly ONE relevant event,
//      rendered in a compact single-line "Calendar signal:" form.
//   4. Private/Busy events are SKIPPED — they never reach the
//      relevance scorer. The block must never reveal or imply a
//      private title, and must never say "you have something on
//      your calendar".
//
// CARRY-FORWARD INVARIANTS:
//   - Adapter boundary (v0.6.0C): only summary/start/end can ever reach
//     this module.
//   - Production rank call site is bit-identical to post-v0.6.0C state.
//     This module is consumed by the offline real-model dry-run script
//     (apps/fomo/scripts/ops-calendar-real-model-dryrun.ts) and unit
//     tests. v0.6.0E.2 is the separate gate that wires it into the
//     production worker.

import type { RankerEgressView } from '../core/egress-policy.js';
import { type CalendarContext, type CalendarEvent } from '../adapters/google-calendar/types.js';

/**
 * Phase v0.6.0E.1c — Bumped because the block FORMAT changed materially:
 *   v0.4.0 (v0.6.0D): always renders event list when ctx has events
 *   v0.4.1 (v0.6.0E.1b): same list + long guidance paragraph
 *   v0.4.2 (v0.6.0E.1c): omits entirely unless one relevant event
 *                        matches; renders compact "Calendar signal:"
 *                        single line.
 */
export const PROMPT_VERSION_WITH_CALENDAR = 'ranker-v0.4.2';

/* ====================================================================== */
/* Relevance heuristic (deterministic, pure function)                      */
/* ====================================================================== */

/**
 * Stopwords filtered from token comparison. Two buckets:
 *   - generic English filler that adds no semantic value
 *   - generic email/meeting words that are too low-signal to drive
 *     a relevance match on their own (we don't want "meeting" alone
 *     to rescue Stripe → "vendor sync")
 */
const RELEVANCE_STOPWORDS: ReadonlySet<string> = new Set([
  // generic filler
  'with', 'about', 'from', 'this', 'that', 'your', 'will',
  'have', 'been', 'were', 'they', 'their', 'them',
  'just', 'than', 'then', 'when', 'where', 'what', 'which',
  'over', 'into', 'onto', 'before', 'after', 'while', 'still',
  'must', 'should', 'would', 'could', 'might', 'shall',
  'also', 'only', 'some', 'much', 'many', 'most', 'each',
  'such', 'same', 'well', 'very', 'here', 'there',
  // temporal filler (time-words are weak signals on their own)
  'today', 'tomorrow', 'yesterday',
  'morning', 'afternoon', 'evening', 'tonight',
  'week', 'next', 'last', 'time', 'times', 'date',
  // generic email / meeting words (too low-signal alone)
  'email', 'message', 'reply', 'send', 'sent', 'sending',
  'meeting', 'meetings', 'call', 'calls', 'sync',
  // common email pleasantries
  'thanks', 'hello', 'please', 'reminder'
]);

function extractRelevanceTokens(text: string): Set<string> {
  const tokens = new Set<string>();
  const matches = text.toLowerCase().match(/[a-z0-9]+/g) ?? [];
  for (const t of matches) {
    if (t.length >= 4 && !RELEVANCE_STOPWORDS.has(t)) tokens.add(t);
  }
  return tokens;
}

/**
 * Select the single most-relevant calendar event for this email, or null
 * if no event passes the relevance gate.
 *
 * Relevance signal = count of meaningful (post-stopword) token overlaps
 * between the email (sender_name + subject + body_snippet) and the
 * event's summary. The MEANINGFUL token filter (length ≥ 4 +
 * stopword-excluded) makes single-word overlaps strong signals: a match
 * on "standup", "parents", or "sheila" is informative because the
 * stopword list has already stripped weak overlaps like "meeting"
 * or "today".
 *
 * Pure function. Exported for tests.
 *
 * CONTRACT:
 *   - Returns null when ctx is null or events is empty.
 *   - Skips events with summary === 'Busy' (private events, already
 *     masked at the adapter boundary in v0.6.0C). This is the
 *     load-bearing private-event rule: even when the email IS
 *     scheduling-related, we omit. Calendar acts as a silent prior;
 *     the prompt never says "you have something on your calendar".
 *   - Requires at least one meaningful token overlap. If the best
 *     event scores zero, returns null and the calendar block is
 *     omitted entirely.
 */
export function selectRelevantCalendarEvent(
  view: RankerEgressView,
  ctx: CalendarContext | null
): CalendarEvent | null {
  if (ctx === null || ctx.events.length === 0) return null;

  const senderName = view.sender_name ?? '';
  const senderEmail = view.sender_email ?? '';
  const subject = view.subject ?? '';
  const bodySnippet = view.body_snippet ?? '';
  const emailText = `${senderName} ${senderEmail} ${subject} ${bodySnippet}`;
  const emailTokens = extractRelevanceTokens(emailText);
  if (emailTokens.size === 0) return null;

  let bestEvent: CalendarEvent | null = null;
  let bestScore = 0;

  for (const event of ctx.events) {
    // SKIP private events. v0.6.0C masks visibility=private to
    // summary='Busy'; that mask hits here as the skip signal.
    if (event.summary === 'Busy') continue;

    const eventTokens = extractRelevanceTokens(event.summary);
    if (eventTokens.size === 0) continue;

    let overlap = 0;
    for (const tok of eventTokens) {
      if (emailTokens.has(tok)) overlap++;
    }
    if (overlap > bestScore) {
      bestScore = overlap;
      bestEvent = event;
    }
  }

  if (bestScore >= 1) return bestEvent;
  return null;
}

/* ====================================================================== */
/* Calendar block builder                                                  */
/* ====================================================================== */

/**
 * Build the Calendar context block for the ranker prompt.
 *
 *   - Returns '' when no event passes the relevance gate (this is the
 *     overwhelmingly common case). The model sees no Calendar block
 *     at all — empty/irrelevant calendars influence nothing.
 *   - Returns a single-line "Calendar signal: …" string when there is
 *     a directly-relevant event. Format anchored to the founder's
 *     v0.6.0E.1c template.
 *
 * Format:
 *   `Calendar signal: This email appears related to a calendar event
 *    <offset>: <summary>. Use this only as timing context.`
 *
 * `now` is a clock injection for tests + the eval (offsets are
 * computed relative to it). Production callers pass Date.now.
 */
export function buildCalendarContextBlock(
  view: RankerEgressView,
  ctx: CalendarContext | null,
  now: () => number = Date.now
): string {
  const relevantEvent = selectRelevantCalendarEvent(view, ctx);
  if (relevantEvent === null) return '';

  const nowMs = now();
  const startMs = Date.parse(relevantEvent.start);
  const offset = Number.isFinite(startMs) ? formatOffset(startMs - nowMs) : 'soon';

  return `Calendar signal: This email appears related to a calendar event ${offset}: ${relevantEvent.summary}. Use this only as timing context.`;
}

/**
 * Format a millisecond delta as a short, founder-readable offset.
 *   - in-progress (delta <= 0): "now"
 *   - < 60 minutes ahead: "in NNm"
 *   - 60 minutes or more: "in NNh" (whole hours) with optional "NNm" tail
 *     omitted for terseness past the 24h mark
 *
 * Pure helper, exported for unit tests.
 */
export function formatOffset(deltaMs: number): string {
  if (deltaMs <= 0) return 'now';
  const totalMinutes = Math.round(deltaMs / 60_000);
  if (totalMinutes < 60) return `in ${totalMinutes}m`;
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes - hours * 60;
  if (hours >= 24) return `in ${hours}h`;
  if (minutes === 0) return `in ${hours}h`;
  return `in ${hours}h ${minutes}m`;
}
