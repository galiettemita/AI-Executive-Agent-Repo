import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  BREVIO_FEEDBACK_ACTIVE_SURFACES,
  BREVIO_FEEDBACK_EVENT_KINDS,
  BREVIO_FEEDBACK_SURFACES,
  BrevioFeedbackError,
  FEEDBACK_EVENT_KINDS,
  InMemoryFeedbackStore,
  isActiveFeedbackSurface,
  isBrevioFeedbackEventKind,
  isBrevioFeedbackSurface,
  isFeedbackEventKind,
  mapLegacyFeedbackKind,
  resolveAndGateSourceSurface,
  resolveFeedbackVerb
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

/* ====================================================================== */
/* Phase v0.5.9 — Brevio-wide Feedback + Learn/Grow Loop substrate          */
/* ====================================================================== */

describe('v0.5.9 BREVIO_FEEDBACK_SURFACES + active allowlist (Q2.A)', () => {
  it('declares exactly the 13 future surfaces in locked order', () => {
    assert.equal(BREVIO_FEEDBACK_SURFACES.length, 13);
    assert.deepEqual([...BREVIO_FEEDBACK_SURFACES], [
      'email_alert',
      'calendar_reminder',
      'draft_suggestion',
      'task_update',
      'stock_watch',
      'coffee_routine',
      'travel_signal',
      'tool_result',
      'browser_summary',
      'booking_preparation',
      'payment_preparation',
      'memory_explanation',
      'why_answer'
    ]);
  });

  it('BREVIO_FEEDBACK_ACTIVE_SURFACES is exactly [email_alert] in v0.5.9', () => {
    assert.equal(BREVIO_FEEDBACK_ACTIVE_SURFACES.length, 1);
    assert.equal(BREVIO_FEEDBACK_ACTIVE_SURFACES[0], 'email_alert');
  });

  it('isBrevioFeedbackSurface recognizes all 13 declared surfaces', () => {
    for (const s of BREVIO_FEEDBACK_SURFACES) {
      assert.equal(isBrevioFeedbackSurface(s), true);
    }
  });

  it('isBrevioFeedbackSurface rejects undeclared values', () => {
    assert.equal(isBrevioFeedbackSurface('not_a_real_surface'), false);
    assert.equal(isBrevioFeedbackSurface(''), false);
    assert.equal(isBrevioFeedbackSurface(null), false);
    assert.equal(isBrevioFeedbackSurface(42), false);
  });

  it('isActiveFeedbackSurface accepts email_alert only; rejects declared-but-inactive', () => {
    assert.equal(isActiveFeedbackSurface('email_alert'), true);
    assert.equal(isActiveFeedbackSurface('calendar_reminder'), false);
    assert.equal(isActiveFeedbackSurface('draft_suggestion'), false);
    assert.equal(isActiveFeedbackSurface('not_a_real_surface'), false);
  });
});

describe('v0.5.9 BREVIO_FEEDBACK_EVENT_KINDS (Q3.A-modified)', () => {
  it('declares exactly the 6 locked generic verbs', () => {
    assert.equal(BREVIO_FEEDBACK_EVENT_KINDS.length, 6);
    assert.deepEqual([...BREVIO_FEEDBACK_EVENT_KINDS].sort(), [
      'approved',
      'asked_why',
      'corrected',
      'ignored',
      'rejected',
      'snoozed'
    ]);
  });

  it('does NOT include opened in v0.5.9 (no current caller; founder lock)', () => {
    assert.equal((BREVIO_FEEDBACK_EVENT_KINDS as readonly string[]).includes('opened'), false);
  });

  it('isBrevioFeedbackEventKind recognizes generic verbs and rejects legacy/unknown', () => {
    assert.equal(isBrevioFeedbackEventKind('ignored'), true);
    assert.equal(isBrevioFeedbackEventKind('approved'), true);
    assert.equal(isBrevioFeedbackEventKind('founder_approved'), false);
    assert.equal(isBrevioFeedbackEventKind('stop'), false);
    assert.equal(isBrevioFeedbackEventKind('not_real'), false);
  });
});

describe('v0.5.9 mapLegacyFeedbackKind (Q3.A-modified compatibility map)', () => {
  it('maps founder_approved → {verb: approved, role: founder}', () => {
    const mapped = mapLegacyFeedbackKind('founder_approved');
    assert.ok(mapped);
    assert.equal(mapped.verb, 'approved');
    assert.equal(mapped.overlay.role, 'founder');
    assert.equal(mapped.overlay.dimension, undefined);
  });

  it('maps founder_rejected → {verb: rejected, role: founder}', () => {
    const mapped = mapLegacyFeedbackKind('founder_rejected');
    assert.ok(mapped);
    assert.equal(mapped.verb, 'rejected');
    assert.equal(mapped.overlay.role, 'founder');
  });

  it('maps user_snoozed → {verb: snoozed, role: user, dimension: alert}', () => {
    const mapped = mapLegacyFeedbackKind('user_snoozed');
    assert.ok(mapped);
    assert.equal(mapped.verb, 'snoozed');
    assert.equal(mapped.overlay.role, 'user');
    assert.equal(mapped.overlay.dimension, 'alert');
  });

  it('maps user_ignored → {verb: ignored, role: user, dimension: alert}', () => {
    const mapped = mapLegacyFeedbackKind('user_ignored');
    assert.ok(mapped);
    assert.equal(mapped.verb, 'ignored');
    assert.equal(mapped.overlay.dimension, 'alert');
  });

  it('maps ignored_sender → {verb: ignored, role: user, dimension: sender} — THE Q5.B TRIGGER', () => {
    const mapped = mapLegacyFeedbackKind('ignored_sender');
    assert.ok(mapped);
    assert.equal(mapped.verb, 'ignored');
    assert.equal(mapped.overlay.dimension, 'sender');
    assert.equal(mapped.overlay.role, 'user');
  });

  it('maps asked_why → {verb: asked_why, role: user}', () => {
    const mapped = mapLegacyFeedbackKind('asked_why');
    assert.ok(mapped);
    assert.equal(mapped.verb, 'asked_why');
  });

  it('maps no_response → {verb: ignored, role: user, reason: no_response}', () => {
    const mapped = mapLegacyFeedbackKind('no_response');
    assert.ok(mapped);
    assert.equal(mapped.verb, 'ignored');
    assert.equal(mapped.overlay.reason, 'no_response');
  });

  it('maps false_positive → {verb: corrected, role: founder, dimension: ranker_label, previous_label: positive}', () => {
    const mapped = mapLegacyFeedbackKind('false_positive');
    assert.ok(mapped);
    assert.equal(mapped.verb, 'corrected');
    assert.equal(mapped.overlay.dimension, 'ranker_label');
    assert.equal(mapped.overlay.previous_label, 'positive');
  });

  it('maps false_negative → {verb: corrected, role: founder, dimension: ranker_label, previous_label: negative}', () => {
    const mapped = mapLegacyFeedbackKind('false_negative');
    assert.ok(mapped);
    assert.equal(mapped.verb, 'corrected');
    assert.equal(mapped.overlay.previous_label, 'negative');
  });

  it('does NOT map stop — founder lock: consent/control stays separate from preference', () => {
    assert.equal(mapLegacyFeedbackKind('stop'), null);
  });

  it('does NOT map user_opened — no current caller; opened kind deferred from v0.5.9', () => {
    assert.equal(mapLegacyFeedbackKind('user_opened'), null);
  });

  it('returns null for generic kinds (no mapping needed)', () => {
    assert.equal(mapLegacyFeedbackKind('ignored'), null);
    assert.equal(mapLegacyFeedbackKind('approved'), null);
  });

  it('returns null for unknown strings', () => {
    assert.equal(mapLegacyFeedbackKind('not_a_real_kind'), null);
    assert.equal(mapLegacyFeedbackKind(''), null);
  });

  it('counts 10 mappable legacy kinds (the founder-locked Q3.A-modified compat map)', () => {
    const mappable = FEEDBACK_EVENT_KINDS.filter((k) => mapLegacyFeedbackKind(k) !== null);
    assert.equal(mappable.length, 9); // 11 legacy - stop - user_opened = 9
    // (user_opened is in the legacy enum but unmapped per founder lock; stop too)
    const expected = [
      'founder_approved',
      'founder_rejected',
      'user_snoozed',
      'user_ignored',
      'ignored_sender',
      'asked_why',
      'no_response',
      'false_positive',
      'false_negative'
    ];
    assert.deepEqual([...mappable].sort(), [...expected].sort());
  });
});

describe('v0.5.9 resolveFeedbackVerb', () => {
  it('passes through generic verbs unchanged', () => {
    assert.equal(resolveFeedbackVerb('ignored'), 'ignored');
    assert.equal(resolveFeedbackVerb('approved'), 'approved');
  });
  it('returns mapped verb for legacy kinds', () => {
    assert.equal(resolveFeedbackVerb('founder_approved'), 'approved');
    assert.equal(resolveFeedbackVerb('ignored_sender'), 'ignored');
  });
  it('returns null for unmappable kinds', () => {
    assert.equal(resolveFeedbackVerb('stop'), null);
    assert.equal(resolveFeedbackVerb('user_opened'), null);
    assert.equal(resolveFeedbackVerb('not_real'), null);
  });
});

describe('v0.5.9 BrevioFeedbackError', () => {
  it('carries code + sanitized attempted_source_surface', () => {
    const err = new BrevioFeedbackError('inactive_surface', 'calendar_reminder');
    assert.equal(err.code, 'inactive_surface');
    assert.equal(err.attempted_source_surface, 'calendar_reminder');
    assert.equal(err.name, 'BrevioFeedbackError');
    assert.ok(err.message.includes('inactive_surface'));
    assert.ok(err.message.includes('calendar_reminder'));
  });

  it('truncates hostile-length attempted values to 64 chars', () => {
    const longString = 'A'.repeat(10_000);
    const err = new BrevioFeedbackError('unknown_surface', longString);
    assert.equal(err.attempted_source_surface.length, 64);
  });
});

describe('v0.5.9 resolveAndGateSourceSurface', () => {
  it('defaults to email_alert when source_surface omitted', () => {
    const resolved = resolveAndGateSourceSurface({
      user_id: 'u1', alert_id: null, sender_email: null, kind: 'founder_approved'
    });
    assert.equal(resolved, 'email_alert');
  });

  it('accepts explicit email_alert', () => {
    const resolved = resolveAndGateSourceSurface({
      user_id: 'u1', alert_id: null, sender_email: null, kind: 'founder_approved',
      source_surface: 'email_alert'
    });
    assert.equal(resolved, 'email_alert');
  });

  it('REJECTS declared-but-inactive surface (calendar_reminder) with inactive_surface — LOAD-BEARING "not trapped in email" proof', () => {
    try {
      resolveAndGateSourceSurface({
        user_id: 'u1', alert_id: null, sender_email: null, kind: 'ignored',
        source_surface: 'calendar_reminder'
      });
      assert.fail('expected BrevioFeedbackError to be thrown');
    } catch (err) {
      assert.ok(err instanceof BrevioFeedbackError);
      assert.equal((err as BrevioFeedbackError).code, 'inactive_surface');
      assert.equal((err as BrevioFeedbackError).attempted_source_surface, 'calendar_reminder');
    }
  });

  it('REJECTS unknown surface with unknown_surface', () => {
    try {
      resolveAndGateSourceSurface({
        user_id: 'u1', alert_id: null, sender_email: null, kind: 'ignored',
        source_surface: 'not_a_real_surface'
      });
      assert.fail('expected BrevioFeedbackError to be thrown');
    } catch (err) {
      assert.ok(err instanceof BrevioFeedbackError);
      assert.equal((err as BrevioFeedbackError).code, 'unknown_surface');
    }
  });

  it('REJECTS every declared-but-inactive surface (12 surfaces) with inactive_surface', () => {
    const inactiveSurfaces = BREVIO_FEEDBACK_SURFACES.filter((s) => s !== 'email_alert');
    assert.equal(inactiveSurfaces.length, 12);
    for (const s of inactiveSurfaces) {
      try {
        resolveAndGateSourceSurface({
          user_id: 'u1', alert_id: null, sender_email: null, kind: 'ignored',
          source_surface: s
        });
        assert.fail(`expected BrevioFeedbackError(inactive_surface) for ${s}`);
      } catch (err) {
        assert.ok(err instanceof BrevioFeedbackError, `expected BrevioFeedbackError for ${s}, got ${err}`);
        assert.equal((err as BrevioFeedbackError).code, 'inactive_surface');
      }
    }
  });
});

describe('v0.5.9 InMemoryFeedbackStore.write — active-surface gate', () => {
  it('accepts source_surface=email_alert (default) and returns the written event with id', async () => {
    const store = new InMemoryFeedbackStore();
    const event = await store.write({
      user_id: 'u1', alert_id: 'a1', sender_email: 'foo@example.com',
      kind: 'founder_approved'
    });
    assert.equal(event.source_surface, 'email_alert');
    assert.equal(event.kind, 'founder_approved'); // literal storage, not mapped
    assert.ok(event.id);
  });

  it('REJECTS source_surface=calendar_reminder; no row written — LOAD-BEARING smoke C6 in unit form', async () => {
    const store = new InMemoryFeedbackStore();
    await assert.rejects(
      store.write({
        user_id: 'u1', alert_id: null, sender_email: 'foo@example.com',
        kind: 'ignored',
        source_surface: 'calendar_reminder'
      }),
      (err) => err instanceof BrevioFeedbackError && err.code === 'inactive_surface'
    );
    const recent = await store.recent('u1');
    assert.equal(recent.length, 0);
  });

  it('REJECTS source_surface=not_a_real_surface; no row written', async () => {
    const store = new InMemoryFeedbackStore();
    await assert.rejects(
      store.write({
        user_id: 'u1', alert_id: null, sender_email: 'foo@example.com',
        kind: 'ignored',
        source_surface: 'not_a_real_surface'
      }),
      (err) => err instanceof BrevioFeedbackError && err.code === 'unknown_surface'
    );
    const recent = await store.recent('u1');
    assert.equal(recent.length, 0);
  });

  it('persists source_surface in the returned event', async () => {
    const store = new InMemoryFeedbackStore();
    const event = await store.write({
      user_id: 'u1', alert_id: null, sender_email: 'foo@example.com',
      kind: 'ignored',
      source_surface: 'email_alert',
      detail: { dimension: 'sender' }
    });
    assert.equal(event.source_surface, 'email_alert');
    assert.equal((event.detail as Record<string, unknown>).dimension, 'sender');
  });

  it('countByKind continues to work on legacy kinds AND generic kinds (storage is literal)', async () => {
    const store = new InMemoryFeedbackStore();
    await store.write({ user_id: 'u1', alert_id: null, sender_email: null, kind: 'founder_approved' });
    await store.write({ user_id: 'u1', alert_id: null, sender_email: null, kind: 'ignored', detail: { dimension: 'sender' } });
    assert.equal(await store.countByKind('u1', 'founder_approved'), 1);
    assert.equal(await store.countByKind('u1', 'ignored'), 1);
    assert.equal(await store.countByKind('u1', 'founder_rejected'), 0);
  });

  it('cross-tenant: write for u1 does NOT appear in u2 recent (carry-forward isolation)', async () => {
    const store = new InMemoryFeedbackStore();
    await store.write({ user_id: 'u1', alert_id: null, sender_email: 'foo@example.com', kind: 'ignored', detail: { dimension: 'sender' } });
    assert.equal((await store.recent('u1')).length, 1);
    assert.equal((await store.recent('u2')).length, 0);
  });
});
