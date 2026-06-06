// Phase v0.5.9 — feedback-apply consumer tests.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAuditStore } from '../core/audit.ts';
import {
  applyFeedback,
  hashSenderKey,
  normalizeEmailForHash,
  loadSenderHashKey
} from './feedback-apply.ts';
import {
  BREVIO_FEEDBACK_SURFACES,
  type FeedbackEvent
} from './feedback-events.ts';
import { InMemoryMemorySignalStore } from './memory-signals.ts';

const TEST_HASH_KEY = Buffer.from(
  '00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff',
  'hex'
);

function makeEvent(over: Partial<FeedbackEvent> = {}): FeedbackEvent {
  // Use `in` checks so explicit `null` overrides don't get defaulted back to
  // the placeholder values.
  return {
    id: 'id' in over ? over.id : 1,
    occurred_at: over.occurred_at ?? '2026-06-06T17:00:00.000Z',
    user_id: over.user_id ?? 'u1',
    alert_id: 'alert_id' in over ? (over.alert_id ?? null) : null,
    sender_email: 'sender_email' in over ? (over.sender_email ?? null) : 'foo@example.com',
    kind: over.kind ?? 'ignored',
    source_surface: over.source_surface ?? 'email_alert',
    detail: 'detail' in over ? (over.detail ?? null) : { dimension: 'sender' }
  };
}

describe('normalizeEmailForHash', () => {
  it('lowercases + trims', () => {
    assert.equal(normalizeEmailForHash('  Foo@Example.COM  '), 'foo@example.com');
  });
  it('returns empty string for null/undefined/non-string', () => {
    assert.equal(normalizeEmailForHash(null), '');
    assert.equal(normalizeEmailForHash(undefined), '');
    assert.equal(normalizeEmailForHash(42 as unknown as string), '');
  });
  it('does NOT strip + aliases (founder lock: alias stripping would merge user expectations)', () => {
    const aliased = normalizeEmailForHash('foo+travel@gmail.com');
    const plain = normalizeEmailForHash('foo@gmail.com');
    assert.notEqual(aliased, plain);
  });
});

describe('hashSenderKey', () => {
  it('produces deterministic 32-hex-char output', () => {
    const h = hashSenderKey('u1', 'foo@example.com', TEST_HASH_KEY);
    assert.equal(h.length, 32);
    assert.ok(/^[0-9a-f]{32}$/.test(h));
    // Determinism — same inputs → same output.
    assert.equal(h, hashSenderKey('u1', 'foo@example.com', TEST_HASH_KEY));
  });

  it('produces DIFFERENT hashes for the same email under different user_ids (cross-user enumeration block)', () => {
    const hA = hashSenderKey('u1', 'foo@example.com', TEST_HASH_KEY);
    const hB = hashSenderKey('u2', 'foo@example.com', TEST_HASH_KEY);
    assert.notEqual(hA, hB);
  });

  it('produces different hashes for different emails under the same user_id', () => {
    const h1 = hashSenderKey('u1', 'foo@example.com', TEST_HASH_KEY);
    const h2 = hashSenderKey('u1', 'bar@example.com', TEST_HASH_KEY);
    assert.notEqual(h1, h2);
  });

  it('is case-insensitive on the email (via normalize)', () => {
    const lower = hashSenderKey('u1', 'foo@example.com', TEST_HASH_KEY);
    const upper = hashSenderKey('u1', 'FOO@EXAMPLE.COM', TEST_HASH_KEY);
    assert.equal(lower, upper);
  });
});

describe('loadSenderHashKey', () => {
  it('parses hex: prefix', () => {
    const buf = loadSenderHashKey({
      BREVIO_SENDER_HASH_KEY: 'hex:00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff'
    });
    assert.equal(buf.length, 32);
  });
  it('parses base64', () => {
    const base64 = Buffer.alloc(32, 0xab).toString('base64');
    const buf = loadSenderHashKey({ BREVIO_SENDER_HASH_KEY: base64 });
    assert.equal(buf.length, 32);
  });
  it('throws on missing key', () => {
    assert.throws(() => loadSenderHashKey({}), /BREVIO_SENDER_HASH_KEY is required/);
  });
  it('throws on under-length key', () => {
    assert.throws(
      () => loadSenderHashKey({ BREVIO_SENDER_HASH_KEY: 'hex:001122' }),
      /need ≥ 32/
    );
  });
});

describe('applyFeedback — no-match cases', () => {
  it('returns no_match for non-email_alert surface', async () => {
    // Constructing a synthetic event with a non-active surface for the consumer
    // test ONLY — the write path would reject this with BrevioFeedbackError.
    // We bypass the write path to exercise applyFeedback's surface check.
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    const result = await applyFeedback(
      makeEvent({ source_surface: 'calendar_reminder' as 'email_alert' }), // type cast for test only
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY }
    );
    assert.equal(result.kind, 'no_match');
    assert.equal((await memoryStore.list('u1')).length, 0);
    assert.equal((await auditStore.recent('u1')).length, 0);
  });

  it('returns no_match for approved verb (Slack approval path)', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    const result = await applyFeedback(
      makeEvent({ kind: 'founder_approved', detail: null }),
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY }
    );
    assert.equal(result.kind, 'no_match');
    assert.equal((await memoryStore.list('u1')).length, 0);
  });

  it('returns no_match for snoozed (verb=snoozed, dimension=alert)', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    const result = await applyFeedback(
      makeEvent({ kind: 'user_snoozed', detail: { dimension: 'alert' } }),
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY }
    );
    assert.equal(result.kind, 'no_match');
  });

  it('returns no_match for generic ignored without dimension=sender', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    const result = await applyFeedback(
      makeEvent({ kind: 'ignored', detail: { dimension: 'alert' } }),
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY }
    );
    assert.equal(result.kind, 'no_match');
  });

  it('returns no_match when sender_email missing', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    const result = await applyFeedback(
      makeEvent({ kind: 'ignored', sender_email: null, detail: { dimension: 'sender' } }),
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY }
    );
    assert.equal(result.kind, 'no_match');
  });
});

describe('applyFeedback — Q5.B match arm (LOAD-BEARING)', () => {
  it('applies for legacy ignored_sender (overlay-derived dimension)', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    const result = await applyFeedback(
      makeEvent({
        id: 42,
        kind: 'ignored_sender',
        sender_email: 'noisy@example.com',
        detail: null // legacy kind → dimension derived from mapping overlay
      }),
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY, now: () => Date.parse('2026-06-06T17:00:00.000Z') }
    );
    assert.equal(result.kind, 'applied');
    if (result.kind !== 'applied') return;
    assert.equal(result.memory_signal_kind, 'sender_feedback_ignored');
    assert.equal(result.memory_signal_action, 'created');
    assert.equal(result.ignored_count, 1);
    assert.ok(result.confidence > 0.5 && result.confidence < 0.95);

    const signal = await memoryStore.get('u1', 'sender_feedback_ignored', result.scope_key_hash);
    assert.ok(signal);
    const detail = signal.detail as Record<string, unknown>;
    assert.equal(detail.ignored_count, 1);
    assert.equal(detail.source_surface, 'email_alert');
    assert.ok(typeof detail.first_ignored_at === 'string');
    assert.ok(Array.isArray(detail.source_feedback_event_ids));
    assert.deepEqual(detail.source_feedback_event_ids, [42]);
  });

  it('applies for generic ignored with detail.dimension=sender', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    const result = await applyFeedback(
      makeEvent({
        kind: 'ignored',
        sender_email: 'noisy@example.com',
        detail: { dimension: 'sender', role: 'user' }
      }),
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY }
    );
    assert.equal(result.kind, 'applied');
  });

  it('emits brevio.feedback.applied audit with STRUCTURAL-ONLY detail (NO raw sender_email)', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    const result = await applyFeedback(
      makeEvent({
        id: 99,
        kind: 'ignored_sender',
        sender_email: 'noisy-newsletter@example.com',
        detail: null
      }),
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY }
    );
    assert.equal(result.kind, 'applied');
    const recent = await auditStore.recent('u1');
    const appliedRow = recent.find((r) => r.action === 'brevio.feedback.applied');
    assert.ok(appliedRow);
    const detail = appliedRow.detail as Record<string, unknown>;
    assert.equal(detail.source_surface, 'email_alert');
    assert.equal(detail.verb, 'ignored');
    assert.equal(detail.dimension, 'sender');
    assert.equal(detail.memory_signal_kind, 'sender_feedback_ignored');
    assert.equal(detail.memory_signal_action, 'created');
    assert.equal(detail.feedback_event_id, 99);
    assert.ok(typeof detail.memory_signal_scope_key_hash === 'string');
    assert.equal((detail.memory_signal_scope_key_hash as string).length, 32);

    // Privacy canary: raw sender email MUST NOT appear in audit detail.
    const json = JSON.stringify(detail);
    assert.equal(json.includes('noisy-newsletter@example.com'), false);
    assert.equal(json.includes('@example.com'), false);
  });

  it('memory_signal detail contains NO raw sender_email (privacy canary)', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    await applyFeedback(
      makeEvent({ kind: 'ignored_sender', sender_email: 'noisy@example.com', detail: null }),
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY }
    );
    const signals = await memoryStore.list('u1');
    assert.equal(signals.length, 1);
    const json = JSON.stringify(signals[0]);
    assert.equal(json.includes('noisy@example.com'), false);
    assert.equal(json.includes('@example.com'), false);
  });

  it('second event for the same (user, sender) increments ignored_count + updates last_ignored_at + preserves first_ignored_at', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    // First event.
    const r1 = await applyFeedback(
      makeEvent({ id: 1, kind: 'ignored_sender', sender_email: 'noisy@example.com', detail: null, occurred_at: '2026-06-06T17:00:00.000Z' }),
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY, now: () => Date.parse('2026-06-06T17:00:00.000Z') }
    );
    assert.equal(r1.kind, 'applied');
    if (r1.kind !== 'applied') return;
    const firstHash = r1.scope_key_hash;

    // Second event for same sender.
    const r2 = await applyFeedback(
      makeEvent({ id: 2, kind: 'ignored_sender', sender_email: 'noisy@example.com', detail: null, occurred_at: '2026-06-06T18:30:00.000Z' }),
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY, now: () => Date.parse('2026-06-06T18:30:00.000Z') }
    );
    assert.equal(r2.kind, 'applied');
    if (r2.kind !== 'applied') return;
    assert.equal(r2.memory_signal_action, 'updated');
    assert.equal(r2.ignored_count, 2);
    assert.equal(r2.scope_key_hash, firstHash); // same hash for same (user, sender)
    assert.ok(r2.confidence > r1.confidence); // confidence increments

    const signal = await memoryStore.get('u1', 'sender_feedback_ignored', firstHash);
    assert.ok(signal);
    const detail = signal.detail as Record<string, unknown>;
    assert.equal(detail.ignored_count, 2);
    assert.equal(detail.first_ignored_at, '2026-06-06T17:00:00.000Z');
    assert.equal(detail.last_ignored_at, '2026-06-06T18:30:00.000Z');
    assert.deepEqual(detail.source_feedback_event_ids, [1, 2]);
  });

  it('reversibility (C10): DELETE memory_signal row → next event recreates with ignored_count=1 (not resumed)', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    const r1 = await applyFeedback(
      makeEvent({ kind: 'ignored_sender', sender_email: 'noisy@example.com', detail: null }),
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY }
    );
    assert.equal(r1.kind, 'applied');
    if (r1.kind !== 'applied') return;
    const hash = r1.scope_key_hash;

    // Delete.
    const deleted = await memoryStore.delete('u1', 'sender_feedback_ignored', hash);
    assert.equal(deleted, true);

    // Re-apply.
    const r2 = await applyFeedback(
      makeEvent({ kind: 'ignored_sender', sender_email: 'noisy@example.com', detail: null }),
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY }
    );
    assert.equal(r2.kind, 'applied');
    if (r2.kind !== 'applied') return;
    assert.equal(r2.memory_signal_action, 'created'); // fresh
    assert.equal(r2.ignored_count, 1);
  });

  it('cross-tenant (C11): apply for user A does NOT touch user B memory_signals', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    await applyFeedback(
      makeEvent({ user_id: 'uA', kind: 'ignored_sender', sender_email: 'shared@example.com', detail: null }),
      { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY }
    );
    assert.equal((await memoryStore.list('uA')).length, 1);
    assert.equal((await memoryStore.list('uB')).length, 0);
  });

  it('source_feedback_event_ids capped at 100 (memory bound)', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    // Drive 120 applies for the same (user, sender).
    for (let i = 0; i < 120; i++) {
      await applyFeedback(
        makeEvent({ id: i + 1, kind: 'ignored_sender', sender_email: 'noisy@example.com', detail: null }),
        { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY }
      );
    }
    const list = await memoryStore.list('uA');
    void list; // suppress unused
    // Reach into the single signal row.
    const signals = await memoryStore.list('u1');
    assert.equal(signals.length, 1);
    const detail = signals[0]!.detail as Record<string, unknown>;
    assert.equal(detail.ignored_count, 120);
    const ids = detail.source_feedback_event_ids as number[];
    assert.equal(ids.length, 100);
    // Last 100 IDs preserved (21..120).
    assert.equal(ids[0], 21);
    assert.equal(ids[99], 120);
  });

  it('confidence is deterministic: min(0.5 + 0.1 * ignored_count, 0.95)', async () => {
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();
    const deps = { memoryStore, auditStore, senderHashKey: TEST_HASH_KEY };
    const expected = [0.6, 0.7, 0.8, 0.9, 0.95, 0.95, 0.95];
    for (let i = 0; i < expected.length; i++) {
      const r = await applyFeedback(
        makeEvent({ id: i + 1, kind: 'ignored_sender', sender_email: 'noisy@example.com', detail: null }),
        deps
      );
      assert.equal(r.kind, 'applied');
      if (r.kind !== 'applied') return;
      assert.ok(
        Math.abs(r.confidence - expected[i]!) < 1e-9,
        `expected confidence=${expected[i]} at i=${i}, got ${r.confidence}`
      );
    }
  });
});

describe('v0.5.9 declared-but-inactive surfaces stay declared', () => {
  it('BREVIO_FEEDBACK_SURFACES still contains all 12 future surfaces (substrate not trapped in email)', () => {
    const future = [
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
    ];
    for (const s of future) {
      assert.ok((BREVIO_FEEDBACK_SURFACES as readonly string[]).includes(s), `expected ${s} declared`);
    }
  });
});
