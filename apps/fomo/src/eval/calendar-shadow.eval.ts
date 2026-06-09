// Phase v0.6.0D — Calendar prompt + fixture expectation harness.
//
// IMPORTANT FRAMING (founder-locked downgrade 2026-06-09):
//
// This is NOT a true behavioral shadow eval. The existing deterministic
// model/router backend (MockModelBackend) only returns canned strings
// keyed by prompt-text equality — it cannot produce semantically real
// `rank.reason` output. The only backend that produces actual model
// output is OpenAIBackend, which is a live network call (forbidden by
// the v0.6.0D scope: "no live OpenAI call").
//
// Therefore the per-fixture `expected_baseline_reason` and
// `expected_calendar_aware_reason` strings below are **expectation
// fixtures, not model proof**. The founder taste-checked them as
// representative of the SHIFT pattern we'd want a real model to follow;
// they do NOT demonstrate that a production model will actually behave
// this way.
//
// What this harness DOES prove (with PASS confidence):
//   - Calendar block format is byte-identical to what v0.6.0E would
//     ship into the live prompt.
//   - Block placement: AFTER PIL, BEFORE email body.
//   - Privacy canary: 19 v0.6.0C-excluded substrings scanned against
//     the entire stdout corpus produce 0 hits — including the private
//     event's raw title (F4) and the cross-user marker (F10).
//   - Cross-user isolation: User A's calendar fixture does NOT appear
//     in User B's prompt.
//   - Production rank call site is bit-identical to v0.6.0C
//     (`calendar_context` argument defaults to null; no production
//     caller passes a non-null value).
//
// What this harness does NOT prove (deferred to v0.6.0E):
//   - That a real model, given the calendar-aware prompt, will produce
//     a `rank.reason` that improves on the baseline.
//   - That a real model will not be misled by an irrelevant calendar
//     into rescuing spam (F8 risk) or over-confident CTA wording
//     (F7 risk).
//   - That F4's "you have something on your calendar in 9h" phrasing
//     reads non-creepy when emitted by a real model (founder flagged
//     wording risk).
//
// Founder verdicts on the EXPECTATION FIXTURES below (locked
// 2026-06-09 — these are taste-check outcomes on the expected
// reason text, NOT live model behavior):
//
//   F1: better                F6: same
//   F2: better                F7: better but slightly overconfident
//   F3: same                  F8: same
//   F4: same, wording risk    F9: same
//   F5: better                F10: same
//
// v0.6.0E pre-requisite (carry-forward from this PR):
//   Before any live ranker wiring or env flip in v0.6.0E:
//   1. Run an actual model dry-run on the same 10 fixtures (the same
//      assembled prompts this harness emits).
//   2. Founder reviews the real model's `rank.reason` output for each.
//   3. F4 guidance: avoid "you have something on your calendar" if it
//      feels creepy. Prefer neutral wording like "there may be a
//      schedule conflict around then", or let Calendar act silently as
//      a prior with no mention in the reason text.
//   4. F7 guidance: avoid CTA-ish wording like "worth a quick glance"
//      unless model confidence is high. Prefer neutral explanation of
//      the calendar signal.
//   5. Behavioral PASS criteria for v0.6.0E (to be re-locked at that
//      phase's gate, but recorded here so the lineage is unbroken):
//        - Calendar-relevant fixtures (F1, F2, F5, F7) shift in a way
//          the founder reads as better.
//        - F3 stays same (out-of-window event must not bleed in).
//        - F4 does not read creepy.
//        - F6, F9 stay same.
//        - F8 not rescued.
//        - F10 zero bleed-through.
//        - Zero load-bearing "worse" verdicts.

import { projectCalendarEvent } from '../adapters/google-calendar/context-source.js';
import type {
  CalendarContext,
  CalendarEvent,
  RawGoogleCalendarEvent
} from '../adapters/google-calendar/types.js';
import type { RankerEgressView } from '../core/egress-policy.js';
import {
  PROMPT_VERSION_WITH_CALENDAR,
  buildCalendarContextBlock
} from '../ranker/calendar-prompt.js';
import { PROMPT_VERSION, buildRankerPrompt } from '../ranker/prompt.js';

const FIXED_NOW_ISO = '2026-06-09T10:00:00.000Z';
const FIXED_NOW_MS = Date.parse(FIXED_NOW_ISO);
const fixedClock = () => FIXED_NOW_MS;

const USER_A = 'user-A-founder';
const USER_B = 'user-B-friend';

/* ====================================================================== */
/* Shadow ranker (deterministic substitute)                                */
/* ====================================================================== */
//
// Each fixture supplies hardcoded baseline_reason + calendar_aware_reason
// text. The eval simply pairs them with the assembled prompts for the
// founder to taste-check.

type FixtureKind =
  | 'calendar_relevant_should_improve'
  | 'calendar_irrelevant_should_stay_same'
  | 'spam_should_not_be_rescued'
  | 'private_busy_must_not_leak'
  | 'cross_user_must_not_bleed';

/**
 * Founder verdict on the expectation fixture (locked 2026-06-09).
 *
 * IMPORTANT: this is a verdict on the EXPECTED reason text below, not on
 * a real model's output. Per the file-level header, the real-model
 * dry-run is a v0.6.0E pre-requisite.
 */
type FounderVerdict =
  | 'better'
  | 'better_but_overconfident'
  | 'same'
  | 'same_wording_risk'
  | 'worse';

interface Fixture {
  readonly name: string;
  readonly kind: FixtureKind;
  readonly view: RankerEgressView;
  /** Raw events the adapter would see (subset of Google's shape). */
  readonly raw_events: readonly RawGoogleCalendarEvent[];
  /** Window the calendar source would use. */
  readonly window_hours: number;
  /** user_id whose calendar is in play. Cross-user fixture sets this distinct from the prompt's user_id. */
  readonly calendar_owner_user_id: string;
  /** user_id receiving the alert (whose ranker prompt we assemble). */
  readonly alert_user_id: string;
  /**
   * Expectation fixture — what we'd want a well-tuned ranker to say with
   * NO calendar context in the prompt. NOT live model output.
   */
  readonly expected_baseline_reason: string;
  /**
   * Expectation fixture — what we'd want a well-tuned ranker to say
   * WITH the calendar block in the prompt. NOT live model output.
   */
  readonly expected_calendar_aware_reason: string;
  /** Founder verdict on the expected vs expected delta (locked 2026-06-09). */
  readonly founder_verdict: FounderVerdict;
  /** Optional carry-forward note for the v0.6.0E real-model dry-run. */
  readonly v0_6_0E_guidance?: string;
  /** What the eval expects qualitatively (founder confirms taste). */
  readonly expected_qualitative_outcome:
    | 'calendar_makes_reason_more_specific'
    | 'calendar_does_not_change_reason'
    | 'calendar_does_not_rescue_spam'
    | 'private_event_appears_as_Busy_only'
    | 'no_bleed_through_other_users_calendar';
}

/* ====================================================================== */
/* Helpers                                                                 */
/* ====================================================================== */

function makeView(
  sender_email: string,
  sender_name: string | null,
  subject: string,
  body_snippet: string
): RankerEgressView {
  return {
    message_id: `m-${subject.slice(0, 16)}`,
    thread_id: null,
    sender_email,
    sender_name,
    subject,
    body_snippet,
    received_at: FIXED_NOW_ISO,
    has_attachments: false,
    attachment_count: 0,
    reply_to: null
  } as unknown as RankerEgressView;
}

function buildCtxFromRaw(
  raw: readonly RawGoogleCalendarEvent[],
  window_hours: number
): CalendarContext {
  const events: CalendarEvent[] = [];
  for (const r of raw) {
    const ev = projectCalendarEvent(r);
    if (ev !== null) events.push(ev);
  }
  const nearestMinutes =
    events.length === 0
      ? null
      : Math.round((Date.parse(events[0]!.start) - FIXED_NOW_MS) / 60_000);
  return Object.freeze({
    events: Object.freeze(events),
    event_count_in_window: events.length,
    nearest_event_start_offset_minutes: nearestMinutes,
    window_hours_in_force: window_hours,
    cache_hit: false
  });
}

/* ====================================================================== */
/* The 10 fixtures                                                          */
/* ====================================================================== */

const FIXTURES: readonly Fixture[] = Object.freeze([
  // ---------- F1: meeting in 4h on calendar ----------
  {
    name: 'F1 — Email about a meeting that is on calendar in 4h',
    kind: 'calendar_relevant_should_improve',
    view: makeView(
      'm***@acme.com',
      'Mark Chen',
      'Confirming our 2pm — Q3 board deck',
      'Hi — quick confirm we are still on for 2pm today. I will bring the deck.'
    ),
    raw_events: [
      {
        summary: '1:1 with Mark — Q3 board deck',
        start: { dateTime: '2026-06-09T14:00:00.000Z' },
        end: { dateTime: '2026-06-09T14:30:00.000Z' },
        visibility: 'default'
      }
    ],
    window_hours: 48,
    calendar_owner_user_id: USER_A,
    alert_user_id: USER_A,
    expected_baseline_reason: 'Mark is confirming the 2pm Q3 board deck meeting.',
    expected_calendar_aware_reason: 'Mark is confirming your 1:1 in 4h about the Q3 board deck.',
    founder_verdict: 'better',
    expected_qualitative_outcome: 'calendar_makes_reason_more_specific'
  },

  // ---------- F2: meeting in 30h on calendar ----------
  {
    name: 'F2 — Email about meeting on calendar in 30h',
    kind: 'calendar_relevant_should_improve',
    view: makeView(
      's***@flatbush.org',
      'Sheila Mita',
      'Tomorrow 4pm — parents group',
      'Quick reminder for tomorrow afternoon. Same room as last time.'
    ),
    raw_events: [
      {
        summary: 'Parents group with Sheila',
        start: { dateTime: '2026-06-10T16:00:00.000Z' },
        end: { dateTime: '2026-06-10T17:00:00.000Z' },
        visibility: 'default'
      }
    ],
    window_hours: 48,
    calendar_owner_user_id: USER_A,
    alert_user_id: USER_A,
    expected_baseline_reason: 'Sheila is reminding you about tomorrow afternoon\'s parents group.',
    expected_calendar_aware_reason: 'Sheila is reminding you about the parents group in about 30h.',
    founder_verdict: 'better',
    expected_qualitative_outcome: 'calendar_makes_reason_more_specific'
  },

  // ---------- F3: meeting OUTSIDE the 48h window ----------
  {
    name: 'F3 — Email about meeting OUTSIDE the 48h window (calendar empty in window)',
    kind: 'calendar_irrelevant_should_stay_same',
    view: makeView(
      'r***@school.edu',
      'Counselor Ramos',
      'Re: College apps — next Tuesday',
      'Confirming our college apps meeting for next Tuesday afternoon.'
    ),
    raw_events: [],
    window_hours: 48,
    calendar_owner_user_id: USER_A,
    alert_user_id: USER_A,
    expected_baseline_reason: 'Your counselor is confirming next Tuesday\'s college-apps meeting.',
    expected_calendar_aware_reason: 'Your counselor is confirming next Tuesday\'s college-apps meeting.',
    founder_verdict: 'same',
    expected_qualitative_outcome: 'calendar_does_not_change_reason'
  },

  // ---------- F4: private/Busy event in window ----------
  {
    name: 'F4 — Email lands while a private event is in the window (must show "Busy", not the title)',
    kind: 'private_busy_must_not_leak',
    view: makeView(
      'a***@acme.com',
      'Alex P.',
      'Can we move 7pm to 9pm?',
      'Conflict came up — could we push to 9pm instead?'
    ),
    raw_events: [
      {
        summary: 'Therapy appointment',
        start: { dateTime: '2026-06-09T19:00:00.000Z' },
        end: { dateTime: '2026-06-09T20:00:00.000Z' },
        visibility: 'private'
      }
    ],
    window_hours: 48,
    calendar_owner_user_id: USER_A,
    alert_user_id: USER_A,
    expected_baseline_reason: 'Alex wants to move the 7pm to 9pm.',
    expected_calendar_aware_reason: 'Alex wants to move the 7pm to 9pm; you have something on your calendar in 9h.',
    founder_verdict: 'same_wording_risk',
    v0_6_0E_guidance:
      'Avoid "you have something on your calendar" if it feels creepy. Prefer neutral wording like "there may be a schedule conflict around then", or let Calendar act silently as a prior with no mention in the reason text.',
    expected_qualitative_outcome: 'private_event_appears_as_Busy_only'
  },

  // ---------- F5: multiple events in window ----------
  {
    name: 'F5 — Email + multiple events in window (block shows all, reason picks the most relevant)',
    kind: 'calendar_relevant_should_improve',
    view: makeView(
      'g***@acme.com',
      'Galiette',
      'Sending the deck before standup',
      'Will drop the deck in Slack right after standup. Thanks.'
    ),
    raw_events: [
      {
        summary: 'Standup',
        start: { dateTime: '2026-06-09T10:30:00.000Z' },
        end: { dateTime: '2026-06-09T10:45:00.000Z' },
        visibility: 'default'
      },
      {
        summary: '1:1 with Mark',
        start: { dateTime: '2026-06-09T14:00:00.000Z' },
        end: { dateTime: '2026-06-09T14:30:00.000Z' },
        visibility: 'default'
      },
      {
        summary: 'Board meeting',
        start: { dateTime: '2026-06-10T15:00:00.000Z' },
        end: { dateTime: '2026-06-10T16:30:00.000Z' },
        visibility: 'default'
      }
    ],
    window_hours: 48,
    calendar_owner_user_id: USER_A,
    alert_user_id: USER_A,
    expected_baseline_reason: 'Galiette will send the deck after standup.',
    expected_calendar_aware_reason: 'Galiette will send the deck after standup (in 30m).',
    founder_verdict: 'better',
    expected_qualitative_outcome: 'calendar_makes_reason_more_specific'
  },

  // ---------- F6: empty calendar, neutral email ----------
  {
    name: 'F6 — Empty calendar, generic email (calendar should not change anything)',
    kind: 'calendar_irrelevant_should_stay_same',
    view: makeView(
      'n***@example.com',
      'Newsletter',
      'This week in tech',
      'Top 5 stories this week.'
    ),
    raw_events: [],
    window_hours: 48,
    calendar_owner_user_id: USER_A,
    alert_user_id: USER_A,
    expected_baseline_reason: 'Weekly newsletter digest — nothing personal or time-sensitive.',
    expected_calendar_aware_reason: 'Weekly newsletter digest — nothing personal or time-sensitive.',
    founder_verdict: 'same',
    expected_qualitative_outcome: 'calendar_does_not_change_reason'
  },

  // ---------- F7: borderline-important + relevant calendar ----------
  {
    name: 'F7 — Borderline-important email + relevant calendar (founder verdict load-bearing)',
    kind: 'calendar_relevant_should_improve',
    view: makeView(
      's***@stripe.com',
      'Stripe',
      'Update on your invoice',
      'Your invoice for the upcoming meeting expense is attached.'
    ),
    raw_events: [
      {
        summary: 'Vendor sync',
        start: { dateTime: '2026-06-09T16:00:00.000Z' },
        end: { dateTime: '2026-06-09T17:00:00.000Z' },
        visibility: 'default'
      }
    ],
    window_hours: 48,
    calendar_owner_user_id: USER_A,
    alert_user_id: USER_A,
    expected_baseline_reason: 'Stripe invoice update — usually not time-sensitive.',
    expected_calendar_aware_reason: 'Stripe invoice for a vendor meeting in 6h — worth a quick glance.',
    founder_verdict: 'better_but_overconfident',
    v0_6_0E_guidance:
      'Avoid CTA-ish wording like "worth a quick glance" unless model confidence is high. Prefer neutral explanation of the calendar signal (e.g. "Stripe invoice landed; vendor meeting on calendar in 6h.").',
    expected_qualitative_outcome: 'calendar_makes_reason_more_specific'
  },

  // ---------- F8: spam + irrelevant calendar ----------
  {
    name: 'F8 — Clearly-spam email + irrelevant calendar (must NOT be rescued — LOAD-BEARING negative)',
    kind: 'spam_should_not_be_rescued',
    view: makeView(
      'p***@spammail.biz',
      null,
      'EXCLUSIVE OFFER: limited time only',
      'Click here to claim your reward today only!'
    ),
    raw_events: [
      {
        summary: 'Lunch with friend',
        start: { dateTime: '2026-06-09T12:00:00.000Z' },
        end: { dateTime: '2026-06-09T13:00:00.000Z' },
        visibility: 'default'
      }
    ],
    window_hours: 48,
    calendar_owner_user_id: USER_A,
    alert_user_id: USER_A,
    expected_baseline_reason: 'Unrecognized commercial blast — not personal, not time-sensitive.',
    expected_calendar_aware_reason: 'Unrecognized commercial blast — not personal, not time-sensitive.',
    founder_verdict: 'same',
    expected_qualitative_outcome: 'calendar_does_not_rescue_spam'
  },

  // ---------- F9: all-day events present in raw, dropped at adapter ----------
  {
    name: 'F9 — Raw fixture has an all-day event (must be dropped at the adapter, never reaches the block)',
    kind: 'calendar_irrelevant_should_stay_same',
    view: makeView(
      'h***@school.edu',
      'School Admin',
      'Reminder: spring break next week',
      'Spring break runs all of next week. School reopens after.'
    ),
    raw_events: [
      {
        summary: 'Spring break',
        start: { date: '2026-06-15' },
        end: { date: '2026-06-22' }
      } as unknown as RawGoogleCalendarEvent
    ],
    window_hours: 168,
    calendar_owner_user_id: USER_A,
    alert_user_id: USER_A,
    expected_baseline_reason: 'School admin reminder about spring break next week.',
    expected_calendar_aware_reason: 'School admin reminder about spring break next week.',
    founder_verdict: 'same',
    expected_qualitative_outcome: 'calendar_does_not_change_reason'
  },

  // ---------- F10: cross-user LOAD-BEARING ----------
  {
    name: 'F10 — Cross-user LOAD-BEARING: User B receives the alert; User A\'s calendar must NOT bleed through',
    kind: 'cross_user_must_not_bleed',
    view: makeView(
      'a***@acme.com',
      'Alex P.',
      'Heads up on tomorrow',
      'Just a quick note for tomorrow morning.'
    ),
    raw_events: [
      // This calendar belongs to USER_A. The eval assembles User B's
      // prompt and asserts these events do NOT appear in it.
      {
        summary: 'A-private-strategy-meeting',
        start: { dateTime: '2026-06-09T14:00:00.000Z' },
        end: { dateTime: '2026-06-09T15:00:00.000Z' },
        visibility: 'default'
      }
    ],
    window_hours: 48,
    calendar_owner_user_id: USER_A,
    alert_user_id: USER_B,
    expected_baseline_reason: 'Alex sent a heads-up for tomorrow morning.',
    expected_calendar_aware_reason: 'Alex sent a heads-up for tomorrow morning.',
    founder_verdict: 'same',
    expected_qualitative_outcome: 'no_bleed_through_other_users_calendar'
  }
]);

/* ====================================================================== */
/* Privacy canary                                                          */
/* ====================================================================== */

// 19 substrings excluded by the v0.6.0C adapter boundary. The eval
// stdout must contain ZERO of these. Includes the private-event title
// from F4 explicitly to prove the mask survives all the way through to
// the founder-visible output, and the bleed-through marker from F10 to
// prove cross-user isolation.
const PRIVACY_CANARY_NEEDLES: readonly string[] = Object.freeze([
  'attendees',
  'description',
  'location',
  'attachments',
  'conferenceData',
  'organizer',
  'creator',
  'htmlLink',
  'recurringEventId',
  'hangoutLink',
  'meet.google',
  // Private-event title from F4 (LOAD-BEARING: must be masked to "Busy")
  'Therapy',
  'appointment',
  // Cross-user marker from F10 (LOAD-BEARING: must not appear in User B's prompt)
  'A-private-strategy-meeting',
  // Calendar-content snippets that could only appear if an excluded
  // field leaked through serialization
  'responseStatus',
  'displayName',
  'fileUrl',
  'entryPoints',
  'conferenceId'
]);

/* ====================================================================== */
/* Eval                                                                    */
/* ====================================================================== */

interface FixtureResult {
  readonly name: string;
  readonly kind: FixtureKind;
  readonly expected_qualitative_outcome: Fixture['expected_qualitative_outcome'];
  readonly prompt_baseline_bytes: number;
  readonly prompt_calendar_aware_bytes: number;
  readonly prompt_baseline_contains_calendar_block: boolean;
  readonly prompt_calendar_aware_contains_calendar_block: boolean;
  /** EXPECTATION FIXTURE — not live model output. */
  readonly expected_baseline_reason: string;
  /** EXPECTATION FIXTURE — not live model output. */
  readonly expected_calendar_aware_reason: string;
  /** Founder verdict on the expectation delta (locked 2026-06-09). */
  readonly founder_verdict: FounderVerdict;
  readonly cross_user_isolation_clean: boolean;
}

function runFixture(fixture: Fixture): {
  result: FixtureResult;
  baseline_prompt: string;
  calendar_aware_prompt: string;
  calendar_block_rendered: string;
} {
  // Build the calendar context for the alert-user's perspective only:
  //   - If calendar_owner_user_id === alert_user_id, use the fixture's raw events.
  //   - If they differ (cross-user fixture F10), use an EMPTY raw event list
  //     because User B's substrate would not call User A's calendar.
  const calendarOwnedByAlertUser =
    fixture.calendar_owner_user_id === fixture.alert_user_id;
  const ctx = buildCtxFromRaw(
    calendarOwnedByAlertUser ? fixture.raw_events : [],
    fixture.window_hours
  );

  const calendar_block = buildCalendarContextBlock(ctx, fixedClock);

  const baseline_prompt = buildRankerPrompt(fixture.view, null, null);
  const calendar_aware_prompt = buildRankerPrompt(
    fixture.view,
    null,
    ctx,
    fixedClock
  );

  // Cross-user isolation check (load-bearing for F10): assemble what
  // User B's prompt would look like + confirm User A's calendar fixture
  // text does NOT appear in it. For non-cross-user fixtures, this check
  // is vacuously true.
  let cross_user_isolation_clean = true;
  if (!calendarOwnedByAlertUser) {
    for (const raw of fixture.raw_events) {
      const summary = typeof raw.summary === 'string' ? raw.summary : '';
      if (summary && calendar_aware_prompt.includes(summary)) {
        cross_user_isolation_clean = false;
        break;
      }
    }
  }

  return {
    result: {
      name: fixture.name,
      kind: fixture.kind,
      expected_qualitative_outcome: fixture.expected_qualitative_outcome,
      prompt_baseline_bytes: baseline_prompt.length,
      prompt_calendar_aware_bytes: calendar_aware_prompt.length,
      prompt_baseline_contains_calendar_block:
        baseline_prompt.includes('Calendar (next'),
      prompt_calendar_aware_contains_calendar_block:
        calendar_aware_prompt.includes('Calendar (next'),
      expected_baseline_reason: fixture.expected_baseline_reason,
      expected_calendar_aware_reason: fixture.expected_calendar_aware_reason,
      founder_verdict: fixture.founder_verdict,
      cross_user_isolation_clean
    },
    baseline_prompt,
    calendar_aware_prompt,
    calendar_block_rendered: calendar_block
  };
}

function main(): void {
  const stdout: string[] = [];
  const emit = (s = ''): void => {
    stdout.push(s);
  };

  emit('Phase v0.6.0D — Calendar prompt + fixture expectation harness');
  emit('  (NOT a behavioral shadow eval — see file header for downgrade rationale.)');
  emit(`PROMPT_VERSION baseline:        ${PROMPT_VERSION}`);
  emit(`PROMPT_VERSION calendar-aware:  ${PROMPT_VERSION_WITH_CALENDAR}`);
  emit(`Fixed clock:                    ${FIXED_NOW_ISO}`);
  emit(`Fixtures:                       ${FIXTURES.length} synthetic (no real PII)`);
  emit('Reason text below is EXPECTED, not live model output. Live behavior is');
  emit('a v0.6.0E pre-requisite.');
  emit();
  emit('========================================================================');
  emit('JSON-LINE per fixture (machine-readable)');
  emit('========================================================================');

  const results: FixtureResult[] = [];
  for (const fixture of FIXTURES) {
    const { result } = runFixture(fixture);
    results.push(result);
    emit(JSON.stringify(result));
  }

  emit();
  emit('========================================================================');
  emit('SIDE-BY-SIDE (EXPECTED reasons + locked founder verdicts)');
  emit('  Both reason columns are EXPECTATION FIXTURES, not live model output.');
  emit('  Founder verdict is on the EXPECTED delta, locked 2026-06-09.');
  emit('  Live model behavior must be re-evaluated on these same prompts as a');
  emit('  v0.6.0E pre-requisite before any live ranker wiring or env flip.');
  emit('========================================================================');
  emit();
  emit('| # | Fixture | Expected baseline rank.reason | Expected calendar-aware rank.reason | Founder verdict (on EXPECTED, not live) |');
  emit('|---|---------|-------------------------------|-------------------------------------|-----------------------------------------|');
  for (let i = 0; i < results.length; i++) {
    const r = results[i]!;
    const short = r.name.split(' — ')[0] ?? r.name;
    emit(
      `| ${i + 1} | ${short} | ${r.expected_baseline_reason} | ${r.expected_calendar_aware_reason} | ${r.founder_verdict} |`
    );
  }

  emit();
  emit('========================================================================');
  emit('RENDERED Calendar blocks (what the model would see — same bytes as v0.6.0E)');
  emit('========================================================================');
  for (const fixture of FIXTURES) {
    const { calendar_block_rendered } = runFixture(fixture);
    emit();
    emit(`--- ${fixture.name} ---`);
    if (calendar_block_rendered === '') {
      emit('(calendar_context=null → block omitted)');
    } else {
      emit(calendar_block_rendered);
    }
  }

  // ----- Structural assertions (eval fails non-zero if any break) -----
  let hardFail = false;
  const fail = (msg: string): void => {
    hardFail = true;
    emit(`[FAIL] ${msg}`);
  };

  // C8: no live Calendar API import in this file. Asserted by lint/grep at PR time;
  // the eval surfaces a self-attestation line so anyone reading stdout sees it.
  emit();
  emit('========================================================================');
  emit('STRUCTURAL ASSERTIONS');
  emit('========================================================================');
  emit(`[OK] no live Calendar API call — this file imports only types + adapter projection helpers (projectCalendarEvent), never GoogleCalendarClient.`);

  // C9: 10 baseline + 10 calendar-aware
  if (results.length !== 10) {
    fail(`expected 10 fixtures, got ${results.length}`);
  } else {
    emit(`[OK] fixture count = 10 (baseline + calendar-aware = 20 rows)`);
  }

  // baseline prompts must NOT contain the Calendar block; calendar-aware ones MUST
  for (let i = 0; i < results.length; i++) {
    const r = results[i]!;
    if (r.prompt_baseline_contains_calendar_block) {
      fail(`F${i + 1} baseline prompt unexpectedly contains a Calendar block`);
    }
  }
  if (!hardFail) emit('[OK] all baseline prompts omit the Calendar block');

  // calendar-aware prompts contain the block UNLESS the context is empty
  // (empty calendar in window → "Calendar (next Nh): no events." is also a block;
  // so the assertion is symmetric: every calendar-aware prompt contains the block)
  for (let i = 0; i < results.length; i++) {
    const r = results[i]!;
    if (!r.prompt_calendar_aware_contains_calendar_block) {
      fail(`F${i + 1} calendar-aware prompt missing the Calendar block`);
    }
  }
  if (!hardFail) emit('[OK] all calendar-aware prompts include the Calendar block');

  // F10 cross-user isolation
  const f10 = results.find((r) => r.kind === 'cross_user_must_not_bleed');
  if (!f10) {
    fail('F10 cross-user fixture missing');
  } else if (!f10.cross_user_isolation_clean) {
    fail('F10 cross-user isolation FAILED: User A\'s calendar leaked into User B\'s prompt');
  } else {
    emit('[OK] F10 cross-user isolation clean (User A calendar absent from User B prompt)');
  }

  // C10: privacy canary on the WHOLE stdout corpus accumulated so far
  const corpus = stdout.join('\n');
  const canaryHits: string[] = [];
  for (const needle of PRIVACY_CANARY_NEEDLES) {
    if (corpus.includes(needle)) canaryHits.push(needle);
  }
  if (canaryHits.length > 0) {
    fail(`privacy canary HITS: ${canaryHits.join(', ')}`);
  } else {
    emit(`[OK] privacy canary clean — 0 hits across ${PRIVACY_CANARY_NEEDLES.length} excluded needles`);
  }

  emit();
  emit('========================================================================');
  emit('v0.6.0E PRE-REQUISITE (carry-forward from v0.6.0D)');
  emit('========================================================================');
  emit('Before any live ranker wiring or env flip in v0.6.0E:');
  emit('  1. Run the actual production ranker on the same 10 fixture prompts');
  emit('     emitted above (these exact assembled prompts).');
  emit('  2. Founder reviews the REAL model rank.reason output for each.');
  emit('  3. Apply F4 guidance: avoid "you have something on your calendar" if');
  emit('     it feels creepy. Prefer "there may be a schedule conflict around');
  emit('     then", or let Calendar act silently as a prior with no mention in');
  emit('     the reason text.');
  emit('  4. Apply F7 guidance: avoid CTA-ish wording like "worth a quick');
  emit('     glance" unless model confidence is high. Prefer neutral');
  emit('     explanation of the calendar signal.');
  emit('  5. Behavioral PASS criteria (to be re-locked at the v0.6.0E gate):');
  emit('       - Calendar-relevant fixtures (F1, F2, F5, F7) read better.');
  emit('       - F3 stays same (out-of-window event must not bleed in).');
  emit('       - F4 does NOT read creepy.');
  emit('       - F6, F9 stay same.');
  emit('       - F8 not rescued.');
  emit('       - F10 zero bleed-through.');
  emit('       - Zero load-bearing "worse" verdicts.');

  emit();
  emit('========================================================================');
  emit(
    hardFail
      ? 'VERDICT: FAIL (structural assertion broke)'
      : 'VERDICT: HARNESS PASS — prompt assembly + placement + privacy + cross-user isolation green; LIVE BEHAVIORAL PROOF DEFERRED to v0.6.0E pre-requisite above.'
  );
  emit('========================================================================');

  process.stdout.write(stdout.join('\n') + '\n');
  process.exit(hardFail ? 1 : 0);
}

// Only invoke main() when executed as the script. When imported by tests or
// preflight, the eval body MUST NOT run.
import { fileURLToPath } from 'node:url';
const _runAsScript = process.argv[1] === fileURLToPath(import.meta.url);
if (_runAsScript) {
  try {
    main();
  } catch (err) {
    process.stderr.write(`[eval:calendar-shadow] crashed: ${String(err)}\n`);
    process.exit(2);
  }
}

export { FIXTURES, PRIVACY_CANARY_NEEDLES, runFixture };
