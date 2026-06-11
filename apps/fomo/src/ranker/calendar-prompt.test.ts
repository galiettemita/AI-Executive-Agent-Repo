// Phase v0.6.0E.1c — relevance-gated Calendar prompt block tests.
//
// Covers the founder-locked PASS criteria:
//   C1 buildCalendarContextBlock returns '' when ctx is null
//   C2 buildCalendarContextBlock returns '' when ctx has zero events
//      (no "no events" line — neutral by absence)
//   C3 buildCalendarContextBlock returns '' when no event passes the
//      relevance gate
//   C4 buildCalendarContextBlock SKIPS Busy events (private mask
//      survives — no creepy phrasing leaked into the block)
//   C5 buildCalendarContextBlock renders the compact single-line
//      format when ONE event is relevant
//   C6 buildRankerPrompt with calendar_context=null is byte-identical
//      to the v0.6.0C baseline (regression guard)
//   C7 buildRankerPrompt drops the Calendar block when relevance gate
//      returns null (no empty-block leak)
//   C8 buildRankerPrompt inserts the Calendar block AFTER PIL and
//      BEFORE the email body when the gate returns a relevant event

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
  PROMPT_VERSION_WITH_CALENDAR,
  selectRelevantCalendarEvent
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

function makeView(
  sender_email: string,
  sender_name: string | null,
  subject: string,
  body_snippet: string
): RankerEgressView {
  return {
    message_id: 'm1',
    thread_id: 't1',
    sender_email,
    sender_name,
    subject,
    body_snippet,
    received_at: '2026-06-09T09:55:00.000Z',
    has_attachments: false,
    attachment_count: 0,
    reply_to: null
  } as unknown as RankerEgressView;
}

const GENERIC_VIEW = makeView(
  's***@example.com',
  'Sample Sender',
  'Random subject with no calendar overlap',
  'Generic body snippet, nothing tied to events.'
);

/* ---------- C1: null context → '' ------------------------------------ */

describe('buildCalendarContextBlock — C1 null context', () => {
  it('returns empty string when ctx is null', () => {
    assert.equal(buildCalendarContextBlock(GENERIC_VIEW, null, fixedClock), '');
  });
});

/* ---------- C2: empty events → '' ------------------------------------ */

describe('buildCalendarContextBlock — C2 empty events', () => {
  it('returns empty string when ctx.events is empty (no "no events" line)', () => {
    assert.equal(
      buildCalendarContextBlock(GENERIC_VIEW, makeCtx([], 48), fixedClock),
      '',
      'empty calendar should be neutral by ABSENCE, not by instruction'
    );
  });

  it('returns empty string regardless of windowHours when empty', () => {
    assert.equal(
      buildCalendarContextBlock(GENERIC_VIEW, makeCtx([], 168), fixedClock),
      ''
    );
  });
});

/* ---------- C3: irrelevant events → '' ------------------------------- */

describe('buildCalendarContextBlock — C3 relevance gate', () => {
  it('returns empty string when no event tokens overlap email tokens', () => {
    const view = makeView(
      's***@spammail.biz',
      null,
      'EXCLUSIVE OFFER: limited time only',
      'Click here to claim your reward today only!'
    );
    const events = [
      makeEvent('Lunch with friend', '2026-06-09T12:00:00.000Z', '2026-06-09T13:00:00.000Z')
    ];
    assert.equal(
      buildCalendarContextBlock(view, makeCtx(events, 48), fixedClock),
      '',
      'spam email + irrelevant lunch event must omit Calendar entirely'
    );
  });

  it('returns empty string when only token overlaps are stopwords like "meeting"', () => {
    const view = makeView(
      's***@stripe.com',
      'Stripe',
      'Update on your invoice',
      'Your invoice for the upcoming meeting expense is attached.'
    );
    const events = [
      makeEvent('Vendor sync', '2026-06-09T16:00:00.000Z', '2026-06-09T17:00:00.000Z')
    ];
    assert.equal(
      buildCalendarContextBlock(view, makeCtx(events, 48), fixedClock),
      '',
      'commercial sender with weak overlap (only "meeting" → stopword) must omit Calendar'
    );
  });
});

/* ---------- C4: Busy events are skipped ------------------------------ */

describe('buildCalendarContextBlock — C4 Busy events skipped', () => {
  it('skips Busy events even when email is clearly scheduling-related', () => {
    const view = makeView(
      'a***@acme.com',
      'Alex P.',
      'Can we move 7pm to 9pm?',
      'Conflict came up — could we push to 9pm instead?'
    );
    const rawPrivate = {
      summary: 'Therapy appointment — should never appear',
      start: { dateTime: '2026-06-09T19:00:00.000Z' },
      end: { dateTime: '2026-06-09T20:00:00.000Z' },
      visibility: 'private'
    };
    const projected = projectCalendarEvent(rawPrivate);
    assert.ok(projected);
    const block = buildCalendarContextBlock(view, makeCtx([projected!]), fixedClock);
    assert.equal(
      block,
      '',
      'private events must NEVER render — Busy mask is the skip signal'
    );
  });

  it('renders a non-Busy event when both Busy and a relevant event are present', () => {
    const view = makeView(
      's***@flatbush.org',
      'Sheila Mita',
      'Parents group reminder',
      'See you at the parents group later.'
    );
    const events = [
      makeEvent('Busy', '2026-06-09T19:00:00.000Z', '2026-06-09T20:00:00.000Z'),
      makeEvent('Parents group with Sheila', '2026-06-10T16:00:00.000Z', '2026-06-10T17:00:00.000Z')
    ];
    const block = buildCalendarContextBlock(view, makeCtx(events, 48), fixedClock);
    assert.ok(block.includes('Parents group with Sheila'));
    assert.equal(block.includes('Busy'), false);
  });

  it('block never contains the literal "you have something on your calendar"', () => {
    // Across many synthetic fixtures, the compact format must never
    // utter that phrase. Defense-in-depth: the format string is locked.
    const view = makeView(
      'm***@acme.com',
      'Mark Chen',
      'Confirming our 2pm — Q3 board deck',
      'still on for 2pm today — I will bring the deck.'
    );
    const events = [
      makeEvent('1:1 with Mark — Q3 board deck', '2026-06-09T14:00:00.000Z', '2026-06-09T14:30:00.000Z')
    ];
    const block = buildCalendarContextBlock(view, makeCtx(events, 48), fixedClock);
    assert.equal(
      block.toLowerCase().includes('you have something on your calendar'),
      false
    );
  });
});

/* ---------- C5: compact single-line format --------------------------- */

describe('buildCalendarContextBlock — C5 compact format', () => {
  it('renders the exact founder-locked "Calendar signal:" template', () => {
    const view = makeView(
      'm***@acme.com',
      'Mark Chen',
      'Confirming our 2pm — Q3 board deck',
      'Hi — quick confirm we are still on for 2pm today. I will bring the deck.'
    );
    const events = [
      makeEvent('1:1 with Mark — Q3 board deck', '2026-06-09T14:00:00.000Z', '2026-06-09T14:30:00.000Z')
    ];
    const block = buildCalendarContextBlock(view, makeCtx(events, 48), fixedClock);
    assert.equal(
      block,
      'Calendar signal: This email appears related to a calendar event in 4h: 1:1 with Mark — Q3 board deck. Use this only as timing context.'
    );
  });

  it('block is a single line (no embedded newlines)', () => {
    const view = makeView(
      's***@flatbush.org',
      'Sheila Mita',
      'Tomorrow 4pm — parents group',
      'Quick reminder for tomorrow afternoon.'
    );
    const events = [
      makeEvent('Parents group with Sheila', '2026-06-10T16:00:00.000Z', '2026-06-10T17:00:00.000Z')
    ];
    const block = buildCalendarContextBlock(view, makeCtx(events, 48), fixedClock);
    assert.equal(block.includes('\n'), false);
    assert.ok(block.startsWith('Calendar signal: '));
    assert.ok(block.endsWith('. Use this only as timing context.'));
  });

  it('picks the highest-overlap event when multiple candidates exist', () => {
    const view = makeView(
      'g***@acme.com',
      'Galiette',
      'Sending the deck before standup',
      'Will drop the deck in Slack right after standup.'
    );
    const events = [
      makeEvent('Standup', '2026-06-09T10:30:00.000Z', '2026-06-09T10:45:00.000Z'),
      makeEvent('1:1 with Mark', '2026-06-09T14:00:00.000Z', '2026-06-09T14:30:00.000Z'),
      makeEvent('Board meeting', '2026-06-10T15:00:00.000Z', '2026-06-10T16:30:00.000Z')
    ];
    const block = buildCalendarContextBlock(view, makeCtx(events, 48), fixedClock);
    assert.ok(block.includes('Standup'));
    assert.equal(block.includes('1:1 with Mark'), false);
    assert.equal(block.includes('Board meeting'), false);
  });
});

/* ---------- selectRelevantCalendarEvent direct contract -------------- */

describe('selectRelevantCalendarEvent — direct contract', () => {
  it('null ctx → null', () => {
    assert.equal(selectRelevantCalendarEvent(GENERIC_VIEW, null), null);
  });

  it('empty events → null', () => {
    assert.equal(selectRelevantCalendarEvent(GENERIC_VIEW, makeCtx([])), null);
  });

  it('no overlap → null', () => {
    assert.equal(
      selectRelevantCalendarEvent(
        GENERIC_VIEW,
        makeCtx([makeEvent('Dentist', '2026-06-09T11:00:00.000Z', '2026-06-09T12:00:00.000Z')])
      ),
      null
    );
  });

  it('Busy-only events → null', () => {
    const view = makeView(
      'a***@acme.com',
      'Alex',
      'About tonight',
      'Question about tonight'
    );
    assert.equal(
      selectRelevantCalendarEvent(
        view,
        makeCtx([makeEvent('Busy', '2026-06-09T19:00:00.000Z', '2026-06-09T20:00:00.000Z')])
      ),
      null
    );
  });

  it('one meaningful overlap is enough', () => {
    const view = makeView(
      'g***@acme.com',
      'Galiette',
      'Sending the deck before standup',
      'Will drop the deck in Slack right after standup.'
    );
    const ev = makeEvent('Standup', '2026-06-09T10:30:00.000Z', '2026-06-09T10:45:00.000Z');
    const picked = selectRelevantCalendarEvent(view, makeCtx([ev]));
    assert.equal(picked, ev);
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
    const view = GENERIC_VIEW;
    const a = buildRankerPrompt(view);
    const b = buildRankerPrompt(view, null);
    const c = buildRankerPrompt(view, null, null);
    assert.equal(a, b);
    assert.equal(b, c);
  });

  it('no Calendar block appears when calendar_context is null', () => {
    const prompt = buildRankerPrompt(GENERIC_VIEW, null, null);
    assert.equal(prompt.includes('Calendar signal'), false);
  });
});

/* ---------- C7: empty-block case is suppressed ----------------------- */

describe('buildRankerPrompt — C7 empty block suppression', () => {
  it('no Calendar block appears when ctx has events but none are relevant', () => {
    const view = GENERIC_VIEW; // no overlap with anything
    const ctx = makeCtx([
      makeEvent('Dentist', '2026-06-09T11:00:00.000Z', '2026-06-09T12:00:00.000Z')
    ]);
    const prompt = buildRankerPrompt(view, null, ctx, fixedClock);
    assert.equal(
      prompt.includes('Calendar signal'),
      false,
      'irrelevant calendar must NOT add a block to the prompt'
    );
  });

  it('no Calendar block appears when only Busy events are in the window', () => {
    const view = makeView(
      'a***@acme.com',
      'Alex P.',
      'Can we move 7pm to 9pm?',
      'Conflict came up.'
    );
    const ctx = makeCtx([
      makeEvent('Busy', '2026-06-09T19:00:00.000Z', '2026-06-09T20:00:00.000Z')
    ]);
    const prompt = buildRankerPrompt(view, null, ctx, fixedClock);
    assert.equal(prompt.includes('Calendar signal'), false);
    assert.equal(prompt.includes('Busy'), false);
  });
});

/* ---------- C8: relevant block goes AFTER PIL, BEFORE body ----------- */

describe('buildRankerPrompt — C8 Calendar block position', () => {
  const pil: PilContext = Object.freeze({
    sender_importance_score: 0.4,
    sender_importance_n_events: 3,
    sender_suppressed: false,
    last_updated: '2026-06-08T10:00:00.000Z',
    decay_factor_applied: 1.0
  });
  const view = makeView(
    'm***@acme.com',
    'Mark Chen',
    'Confirming our 2pm — Q3 board deck',
    'still on for 2pm today — I will bring the deck.'
  );
  const ctx = makeCtx(
    [makeEvent('1:1 with Mark — Q3 board deck', '2026-06-09T14:00:00.000Z', '2026-06-09T14:30:00.000Z')],
    48
  );

  it('Calendar block appears after PIL prior block', () => {
    const prompt = buildRankerPrompt(view, pil, ctx, fixedClock);
    const pilIdx = prompt.indexOf('PIL prior');
    const calIdx = prompt.indexOf('Calendar signal');
    assert.ok(pilIdx >= 0);
    assert.ok(calIdx >= 0);
    assert.ok(pilIdx < calIdx, 'PIL must come before Calendar');
  });

  it('Calendar block appears before the email body section', () => {
    const prompt = buildRankerPrompt(view, pil, ctx, fixedClock);
    const calIdx = prompt.indexOf('Calendar signal');
    const emailIdx = prompt.indexOf('Email to classify:');
    assert.ok(calIdx >= 0);
    assert.ok(emailIdx >= 0);
    assert.ok(calIdx < emailIdx);
  });

  it('Calendar block appears even with PIL absent', () => {
    const prompt = buildRankerPrompt(view, null, ctx, fixedClock);
    const calIdx = prompt.indexOf('Calendar signal');
    const emailIdx = prompt.indexOf('Email to classify:');
    assert.ok(calIdx > 0);
    assert.ok(calIdx < emailIdx);
    assert.equal(prompt.includes('PIL prior'), false);
  });
});

/* ---------- prompt version --------------------------------------- */

describe('PROMPT_VERSION_WITH_CALENDAR', () => {
  it('is the locked v0.6.0E.1c version string', () => {
    assert.equal(PROMPT_VERSION_WITH_CALENDAR, 'ranker-v0.4.2');
  });
});
