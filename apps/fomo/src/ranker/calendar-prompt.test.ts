// Phase v0.6.0D — Calendar prompt block + buildRankerPrompt integration tests.
//
// Covers the founder-locked PASS criteria:
//   C1 buildCalendarContextBlock(null) returns the empty string
//   C2 empty-window context returns the locked "no events" form
//   C3 multi-event context produces the locked 4-line shape
//   C4 private events render as "Busy"; raw title absent
//   C5 all-day events are absent (carry-forward — projectCalendarEvent
//      already rejected the date-shape upstream; tested here for
//      explicit lineage)
//   C6 buildRankerPrompt with calendar_context=null is byte-identical to
//      the v0.6.0C baseline prompt (regression guard)
//   C7 buildRankerPrompt with non-null context appends the Calendar block
//      AFTER the PIL block and BEFORE the email body (Q2.A position lock)

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { projectCalendarEvent } from '../adapters/google-calendar/context-source.ts';
import type {
  CalendarContext,
  CalendarEvent
} from '../adapters/google-calendar/types.ts';
import type { RankerEgressView } from '../core/egress-policy.ts';
import {
  buildCalendarContextBlock,
  formatOffset,
  PROMPT_VERSION_WITH_CALENDAR
} from './calendar-prompt.ts';
import type { PilContext } from './pil-context.ts';
import { buildRankerPrompt } from './prompt.ts';

const FIXED_NOW_MS = Date.parse('2026-06-09T10:00:00.000Z');
const fixedClock = () => FIXED_NOW_MS;

function makeEvent(
  summary: string,
  startIso: string,
  endIso: string
): CalendarEvent {
  return Object.freeze({ summary, start: startIso, end: endIso });
}

function makeCtx(
  events: readonly CalendarEvent[],
  windowH = 48
): CalendarContext {
  return Object.freeze({
    events: Object.freeze([...events]),
    event_count_in_window: events.length,
    nearest_event_start_offset_minutes:
      events.length === 0
        ? null
        : Math.round((Date.parse(events[0]!.start) - FIXED_NOW_MS) / 60_000),
    window_hours_in_force: windowH,
    cache_hit: false
  });
}

const SAMPLE_VIEW: RankerEgressView = Object.freeze({
  message_id: 'm1',
  thread_id: 't1',
  sender_email: 's***@example.com',
  sender_name: 'Sample Sender',
  subject: 'Test subject',
  body_snippet: 'Short body snippet for tests.',
  received_at: '2026-06-09T09:55:00.000Z',
  has_attachments: false,
  attachment_count: 0,
  reply_to: null
} as unknown as RankerEgressView);

/* ---------- C1 ------------------------------------------------------- */

describe('buildCalendarContextBlock — C1 null context', () => {
  it('returns empty string when ctx is null', () => {
    assert.equal(buildCalendarContextBlock(null, fixedClock), '');
  });
});

/* ---------- C2 ------------------------------------------------------- */

describe('buildCalendarContextBlock — C2 empty window', () => {
  it('returns the locked "no events" line with window from context', () => {
    const block = buildCalendarContextBlock(makeCtx([], 48), fixedClock);
    assert.equal(block, 'Calendar (next 48h): no events.');
  });

  it('honors non-default window in the header', () => {
    const block = buildCalendarContextBlock(makeCtx([], 168), fixedClock);
    assert.equal(block, 'Calendar (next 168h): no events.');
  });
});

/* ---------- C3 ------------------------------------------------------- */

describe('buildCalendarContextBlock — C3 multi-event shape', () => {
  it('produces the exact 4-line shape from the v0.6.0D scope', () => {
    const events = [
      makeEvent('1:1 with Galiette', '2026-06-09T14:00:00.000Z', '2026-06-09T14:30:00.000Z'),
      makeEvent('Busy', '2026-06-09T19:00:00.000Z', '2026-06-09T20:00:00.000Z'),
      makeEvent('Board meeting', '2026-06-10T15:00:00.000Z', '2026-06-10T16:30:00.000Z')
    ];
    const block = buildCalendarContextBlock(makeCtx(events, 48), fixedClock);
    assert.equal(
      block,
      [
        'Calendar (next 48h):',
        '  in 4h: 1:1 with Galiette',
        '  in 9h: Busy',
        '  in 29h: Board meeting'
      ].join('\n')
    );
  });

  it('formats minute-scale offsets with "in NNm"', () => {
    const events = [
      makeEvent('Quick sync', '2026-06-09T10:30:00.000Z', '2026-06-09T10:45:00.000Z')
    ];
    const block = buildCalendarContextBlock(makeCtx(events), fixedClock);
    assert.ok(block.includes('  in 30m: Quick sync'));
  });

  it('formats in-progress events as "now"', () => {
    const events = [
      makeEvent('Standup', '2026-06-09T09:55:00.000Z', '2026-06-09T10:15:00.000Z')
    ];
    const block = buildCalendarContextBlock(makeCtx(events), fixedClock);
    assert.ok(block.includes('  now: Standup'));
  });
});

/* ---------- C4 ------------------------------------------------------- */

describe('buildCalendarContextBlock — C4 private events', () => {
  it('renders private events as "Busy" (mask survives at the prompt layer)', () => {
    const rawPrivate = {
      summary: 'Therapy appointment — should never appear',
      start: { dateTime: '2026-06-09T19:00:00.000Z' },
      end: { dateTime: '2026-06-09T20:00:00.000Z' },
      visibility: 'private'
    };
    const projected = projectCalendarEvent(rawPrivate);
    assert.ok(projected);
    const block = buildCalendarContextBlock(makeCtx([projected!]), fixedClock);
    assert.ok(block.includes('Busy'));
    assert.equal(block.includes('Therapy'), false);
    assert.equal(block.includes('appointment'), false);
  });
});

/* ---------- C5 ------------------------------------------------------- */

describe('buildCalendarContextBlock — C5 all-day events absent', () => {
  it('events with date (not dateTime) are dropped at projectCalendarEvent → never reach the block', () => {
    const projected = projectCalendarEvent({
      summary: 'Conference travel day',
      start: { date: '2026-06-11' },
      end: { date: '2026-06-12' }
    });
    assert.equal(projected, null);
    const block = buildCalendarContextBlock(makeCtx([]), fixedClock);
    assert.ok(block.includes('no events'));
    assert.equal(block.includes('Conference'), false);
  });
});

/* ---------- formatOffset edge cases ---------------------------------- */

describe('formatOffset', () => {
  it('past or zero → "now"', () => {
    assert.equal(formatOffset(-1), 'now');
    assert.equal(formatOffset(0), 'now');
  });

  it('sub-hour → "in NNm"', () => {
    assert.equal(formatOffset(15 * 60_000), 'in 15m');
    assert.equal(formatOffset(59 * 60_000), 'in 59m');
  });

  it('exactly 60 min → "in 1h"', () => {
    assert.equal(formatOffset(60 * 60_000), 'in 1h');
  });

  it('sub-day with minutes → "in Xh Ym"', () => {
    assert.equal(formatOffset((9 * 60 + 30) * 60_000), 'in 9h 30m');
  });

  it('past 24h drops the minutes tail', () => {
    assert.equal(formatOffset((29 * 60) * 60_000), 'in 29h');
    assert.equal(formatOffset((29 * 60 + 30) * 60_000), 'in 29h');
  });
});

/* ---------- C6: bit-identical when calendar_context is null/absent --- */

describe('buildRankerPrompt — C6 regression guard', () => {
  it('omitting the calendar argument is byte-identical to passing null', () => {
    const a = buildRankerPrompt(SAMPLE_VIEW);
    const b = buildRankerPrompt(SAMPLE_VIEW, null);
    const c = buildRankerPrompt(SAMPLE_VIEW, null, null);
    assert.equal(a, b);
    assert.equal(b, c);
  });

  it('no Calendar block appears when calendar_context is null', () => {
    const prompt = buildRankerPrompt(SAMPLE_VIEW, null, null);
    assert.equal(prompt.includes('Calendar (next'), false);
  });
});

/* ---------- C7: Calendar block appears AFTER PIL, BEFORE body -------- */

describe('buildRankerPrompt — C7 Calendar block position (Q2.A)', () => {
  const pil: PilContext = Object.freeze({
    sender_importance_score: 0.4,
    sender_importance_n_events: 3,
    sender_suppressed: false,
    last_updated: '2026-06-08T10:00:00.000Z',
    decay_factor_applied: 1.0
  });
  const cal = makeCtx(
    [makeEvent('Demo', '2026-06-09T14:00:00.000Z', '2026-06-09T15:00:00.000Z')],
    48
  );

  it('Calendar block appears after PIL prior block', () => {
    const prompt = buildRankerPrompt(SAMPLE_VIEW, pil, cal, fixedClock);
    const pilIdx = prompt.indexOf('PIL prior');
    const calIdx = prompt.indexOf('Calendar (next');
    assert.ok(pilIdx >= 0, 'PIL block must be present');
    assert.ok(calIdx >= 0, 'Calendar block must be present');
    assert.ok(pilIdx < calIdx, 'PIL must come before Calendar');
  });

  it('Calendar block appears before the email body section', () => {
    const prompt = buildRankerPrompt(SAMPLE_VIEW, pil, cal, fixedClock);
    const calIdx = prompt.indexOf('Calendar (next');
    const emailIdx = prompt.indexOf('Email to classify:');
    assert.ok(calIdx >= 0);
    assert.ok(emailIdx >= 0);
    assert.ok(calIdx < emailIdx, 'Calendar must come before email body');
  });

  it('Calendar block appears even with PIL absent (after examples, before body)', () => {
    const prompt = buildRankerPrompt(SAMPLE_VIEW, null, cal, fixedClock);
    const exampleIdx = prompt.indexOf('Examples of the v0.2.0 reason voice');
    const calIdx = prompt.indexOf('Calendar (next');
    const emailIdx = prompt.indexOf('Email to classify:');
    assert.ok(exampleIdx < calIdx);
    assert.ok(calIdx < emailIdx);
    assert.equal(prompt.includes('PIL prior'), false);
  });
});

/* ---------- prompt version sanity ------------------------------------ */

describe('PROMPT_VERSION_WITH_CALENDAR', () => {
  it('is the locked v0.6.0D version string', () => {
    assert.equal(PROMPT_VERSION_WITH_CALENDAR, 'ranker-v0.4.0');
  });
});
