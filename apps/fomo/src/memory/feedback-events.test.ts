import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  FEEDBACK_EVENT_KINDS,
  InMemoryFeedbackStore,
  isFeedbackEventKind
} from './feedback-events.ts';

describe('FEEDBACK_EVENT_KINDS', () => {
  it('declares exactly the 11 v0.1 event kinds per FOMO_PLAN §9.6', () => {
    assert.equal(FEEDBACK_EVENT_KINDS.length, 11);
    const expected = [
      'founder_approved',
      'founder_rejected',
      'user_opened',
      'user_snoozed',
      'user_ignored',
      'ignored_sender',
      'asked_why',
      'stop',
      'no_response',
      'false_positive',
      'false_negative'
    ];
    assert.deepEqual([...FEEDBACK_EVENT_KINDS].sort(), [...expected].sort());
  });
});

describe('isFeedbackEventKind', () => {
  it('recognizes every declared kind', () => {
    for (const k of FEEDBACK_EVENT_KINDS) {
      assert.equal(isFeedbackEventKind(k), true);
    }
  });

  it('rejects unknown strings and non-strings', () => {
    assert.equal(isFeedbackEventKind('booked_a_flight'), false);
    assert.equal(isFeedbackEventKind(''), false);
    assert.equal(isFeedbackEventKind(null), false);
    assert.equal(isFeedbackEventKind(undefined), false);
    assert.equal(isFeedbackEventKind(42), false);
  });
});

describe('InMemoryFeedbackStore — write + recent', () => {
  it('records and reads back recent events for a user, newest first', async () => {
    const store = new InMemoryFeedbackStore();
    await store.write({
      user_id: 'u1',
      alert_id: 'a1',
      sender_email: 'sarah@school.edu',
      kind: 'founder_approved',
      detail: { score: 0.91 }
    });
    await store.write({
      user_id: 'u1',
      alert_id: 'a1',
      sender_email: 'sarah@school.edu',
      kind: 'user_opened'
    });
    const out = await store.recent('u1');
    assert.equal(out.length, 2);
    assert.equal(out[0]?.kind, 'user_opened');
    assert.equal(out[1]?.kind, 'founder_approved');
  });

  it('isolates events per user', async () => {
    const store = new InMemoryFeedbackStore();
    await store.write({ user_id: 'u1', alert_id: 'a1', sender_email: null, kind: 'founder_approved' });
    await store.write({ user_id: 'u2', alert_id: 'a2', sender_email: null, kind: 'founder_rejected' });
    assert.equal((await store.recent('u1')).length, 1);
    assert.equal((await store.recent('u2')).length, 1);
    assert.equal((await store.recent('u1'))[0]?.kind, 'founder_approved');
    assert.equal((await store.recent('u2'))[0]?.kind, 'founder_rejected');
  });

  it('respects the limit parameter', async () => {
    const store = new InMemoryFeedbackStore();
    for (let i = 0; i < 10; i++) {
      await store.write({ user_id: 'u1', alert_id: `a${i}`, sender_email: null, kind: 'user_opened' });
    }
    const out = await store.recent('u1', 3);
    assert.equal(out.length, 3);
  });

  it('uses provided occurred_at when given', async () => {
    const store = new InMemoryFeedbackStore();
    await store.write({
      user_id: 'u1',
      alert_id: null,
      sender_email: null,
      kind: 'stop',
      occurred_at: '2026-05-22T00:00:00.000Z'
    });
    const [event] = await store.recent('u1');
    assert.equal(event?.occurred_at, '2026-05-22T00:00:00.000Z');
  });

  it('handles events with null alert_id (system-wide events like stop)', async () => {
    const store = new InMemoryFeedbackStore();
    await store.write({ user_id: 'u1', alert_id: null, sender_email: null, kind: 'stop' });
    const [event] = await store.recent('u1');
    assert.equal(event?.alert_id, null);
    assert.equal(event?.kind, 'stop');
  });

  it('respects capacity limit (oldest evicted)', async () => {
    const store = new InMemoryFeedbackStore(3);
    for (let i = 0; i < 5; i++) {
      await store.write({ user_id: 'u1', alert_id: `a${i}`, sender_email: null, kind: 'user_opened' });
    }
    const out = await store.recent('u1');
    assert.equal(out.length, 3);
    // Most recent three are alert_id a2, a3, a4 — returned newest first
    assert.deepEqual(out.map((e) => e.alert_id), ['a4', 'a3', 'a2']);
  });

  it('returned events are frozen', async () => {
    const store = new InMemoryFeedbackStore();
    await store.write({ user_id: 'u1', alert_id: 'a1', sender_email: null, kind: 'founder_approved' });
    const [event] = await store.recent('u1');
    assert.ok(event);
    assert.throws(() => {
      (event as unknown as { kind: string }).kind = 'mutated';
    });
  });
});

describe('InMemoryFeedbackStore — detail redaction', () => {
  it('redacts sensitive keys in detail before persisting (parallels audit log)', async () => {
    const store = new InMemoryFeedbackStore();
    await store.write({
      user_id: 'u1',
      alert_id: 'a1',
      sender_email: null,
      kind: 'founder_approved',
      detail: { score: 0.91, access_token: 'plaintext-leaked' }
    });
    const [event] = await store.recent('u1');
    const detail = event?.detail as Record<string, unknown>;
    assert.equal(detail.access_token, '<redacted>');
    assert.equal(detail.score, 0.91);
  });

  it('handles null detail without error', async () => {
    const store = new InMemoryFeedbackStore();
    await store.write({ user_id: 'u1', alert_id: 'a1', sender_email: null, kind: 'user_opened' });
    const [event] = await store.recent('u1');
    assert.equal(event?.detail, null);
  });

  it('handles undefined detail (treated as null)', async () => {
    const store = new InMemoryFeedbackStore();
    await store.write({ user_id: 'u1', alert_id: 'a1', sender_email: null, kind: 'user_opened', detail: undefined });
    const [event] = await store.recent('u1');
    assert.equal(event?.detail, null);
  });
});

describe('InMemoryFeedbackStore — counters', () => {
  it('countByKind totals across all events for a user', async () => {
    const store = new InMemoryFeedbackStore();
    await store.write({ user_id: 'u1', alert_id: 'a1', sender_email: null, kind: 'founder_approved' });
    await store.write({ user_id: 'u1', alert_id: 'a2', sender_email: null, kind: 'founder_approved' });
    await store.write({ user_id: 'u1', alert_id: 'a3', sender_email: null, kind: 'founder_rejected' });
    await store.write({ user_id: 'u2', alert_id: 'a4', sender_email: null, kind: 'founder_approved' });
    assert.equal(await store.countByKind('u1', 'founder_approved'), 2);
    assert.equal(await store.countByKind('u1', 'founder_rejected'), 1);
    assert.equal(await store.countByKind('u1', 'user_opened'), 0);
    assert.equal(await store.countByKind('u2', 'founder_approved'), 1);
  });

  it('countBySender totals events scoped to a specific sender for a user', async () => {
    const store = new InMemoryFeedbackStore();
    await store.write({ user_id: 'u1', alert_id: 'a1', sender_email: 'sarah@school.edu', kind: 'founder_approved' });
    await store.write({ user_id: 'u1', alert_id: 'a2', sender_email: 'sarah@school.edu', kind: 'user_snoozed' });
    await store.write({ user_id: 'u1', alert_id: 'a3', sender_email: 'noreply@linkedin.com', kind: 'user_ignored' });
    await store.write({ user_id: 'u1', alert_id: 'a4', sender_email: null, kind: 'stop' });
    assert.equal(await store.countBySender('u1', 'sarah@school.edu'), 2);
    assert.equal(await store.countBySender('u1', 'noreply@linkedin.com'), 1);
    assert.equal(await store.countBySender('u1', 'nobody@nowhere.com'), 0);
  });
});
