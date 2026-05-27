import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  InMemoryMemorySignalStore,
  MEMORY_SIGNAL_KINDS,
  MEMORY_SIGNAL_SOURCES,
  defaultConfidence,
  isMemorySignalKind,
  isMemorySignalSource
} from './memory-signals.ts';

describe('MEMORY_SIGNAL_KINDS', () => {
  it('declares the 6 v0.1 personalization signal kinds plus the Phase 3F.1 stop_active compliance signal (7 total)', () => {
    assert.equal(MEMORY_SIGNAL_KINDS.length, 7);
    const expected = [
      'sender_importance',
      'sender_suppressed',
      'timing_preference',
      'topic_importance',
      'alert_usefulness',
      'quietness_preference',
      // Phase 3F.1: TCPA-style STOP enforcement signal. Outbound-sender
      // refuses to dispatch when active=true.
      'stop_active'
    ];
    assert.deepEqual([...MEMORY_SIGNAL_KINDS].sort(), [...expected].sort());
  });
});

describe('MEMORY_SIGNAL_SOURCES', () => {
  it('declares the four provenance sources', () => {
    assert.deepEqual(
      [...MEMORY_SIGNAL_SOURCES].sort(),
      ['feedback_derived', 'founder_set', 'inferred', 'user_confirmed']
    );
  });
});

describe('type guards', () => {
  it('isMemorySignalKind recognizes declared kinds and rejects others', () => {
    for (const k of MEMORY_SIGNAL_KINDS) {
      assert.equal(isMemorySignalKind(k), true);
    }
    assert.equal(isMemorySignalKind('mystery'), false);
    assert.equal(isMemorySignalKind(null), false);
    assert.equal(isMemorySignalKind(42), false);
  });

  it('isMemorySignalSource recognizes declared sources and rejects others', () => {
    for (const s of MEMORY_SIGNAL_SOURCES) {
      assert.equal(isMemorySignalSource(s), true);
    }
    assert.equal(isMemorySignalSource('telepathy'), false);
    assert.equal(isMemorySignalSource(undefined), false);
  });
});

describe('defaultConfidence', () => {
  it('user_confirmed and founder_set are 1.0', () => {
    assert.equal(defaultConfidence('user_confirmed'), 1.0);
    assert.equal(defaultConfidence('founder_set'), 1.0);
  });

  it('feedback_derived is 0.7 (computed from prior events)', () => {
    assert.equal(defaultConfidence('feedback_derived'), 0.7);
  });

  it('inferred is 0.5 (best-effort guess)', () => {
    assert.equal(defaultConfidence('inferred'), 0.5);
  });

  it('feedback_derived is more confident than inferred but less than user_confirmed', () => {
    assert.ok(defaultConfidence('feedback_derived') > defaultConfidence('inferred'));
    assert.ok(defaultConfidence('feedback_derived') < defaultConfidence('user_confirmed'));
  });
});

describe('InMemoryMemorySignalStore — upsert + get', () => {
  it('writes a signal and reads it back', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({
      user_id: 'u1',
      kind: 'sender_importance',
      scope_key: 'sarah@school.edu',
      detail: { importance: 'high' },
      source: 'user_confirmed'
    });
    const s = await store.get('u1', 'sender_importance', 'sarah@school.edu');
    assert.ok(s);
    assert.equal(s?.user_id, 'u1');
    assert.equal(s?.kind, 'sender_importance');
    assert.equal(s?.scope_key, 'sarah@school.edu');
    assert.equal(s?.source, 'user_confirmed');
    assert.equal(s?.confidence, 1.0);
    assert.deepEqual(s?.detail, { importance: 'high' });
  });

  it('returns null for missing signal', async () => {
    const store = new InMemoryMemorySignalStore();
    assert.equal(await store.get('u1', 'sender_importance', 'nobody@nowhere'), null);
  });

  it('upsert replaces existing signal for the same (user, kind, scope_key) key', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({
      user_id: 'u1',
      kind: 'sender_importance',
      scope_key: 'sarah@school.edu',
      detail: { importance: 'medium' },
      source: 'inferred'
    });
    const firstId = (await store.get('u1', 'sender_importance', 'sarah@school.edu'))?.id;
    await store.upsert({
      user_id: 'u1',
      kind: 'sender_importance',
      scope_key: 'sarah@school.edu',
      detail: { importance: 'high' },
      source: 'user_confirmed'
    });
    const s = await store.get('u1', 'sender_importance', 'sarah@school.edu');
    assert.equal((s?.detail as { importance: string }).importance, 'high');
    assert.equal(s?.source, 'user_confirmed');
    assert.equal(s?.confidence, 1.0);
    // ID is preserved across upserts (treat the signal as the same row).
    assert.equal(s?.id, firstId);
  });

  it('upsert with null scope_key handles user-wide preferences', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({
      user_id: 'u1',
      kind: 'quietness_preference',
      scope_key: null,
      detail: { max_per_day: 5 },
      source: 'user_confirmed'
    });
    const s = await store.get('u1', 'quietness_preference');
    assert.ok(s);
    assert.equal(s?.scope_key, null);
    assert.deepEqual(s?.detail, { max_per_day: 5 });
  });
});

describe('InMemoryMemorySignalStore — confidence', () => {
  it('defaults confidence by source when not given', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({
      user_id: 'u1',
      kind: 'sender_importance',
      scope_key: 's1@x',
      detail: {},
      source: 'inferred'
    });
    const s = await store.get('u1', 'sender_importance', 's1@x');
    assert.equal(s?.confidence, 0.5);
  });

  it('explicit confidence overrides the source default', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({
      user_id: 'u1',
      kind: 'sender_importance',
      scope_key: 's1@x',
      detail: {},
      source: 'inferred',
      confidence: 0.9
    });
    const s = await store.get('u1', 'sender_importance', 's1@x');
    assert.equal(s?.confidence, 0.9);
  });

  it('clamps confidence to [0, 1]', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({
      user_id: 'u1',
      kind: 'sender_importance',
      scope_key: 's1@x',
      detail: {},
      source: 'inferred',
      confidence: 5.0
    });
    assert.equal((await store.get('u1', 'sender_importance', 's1@x'))?.confidence, 1.0);

    await store.upsert({
      user_id: 'u1',
      kind: 'sender_importance',
      scope_key: 's2@x',
      detail: {},
      source: 'inferred',
      confidence: -1
    });
    assert.equal((await store.get('u1', 'sender_importance', 's2@x'))?.confidence, 0);
  });
});

describe('InMemoryMemorySignalStore — isolation + listing', () => {
  it('isolates signals per user', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({ user_id: 'u1', kind: 'sender_importance', scope_key: 's@x', detail: {}, source: 'user_confirmed' });
    await store.upsert({ user_id: 'u2', kind: 'sender_importance', scope_key: 's@x', detail: {}, source: 'user_confirmed' });
    assert.equal((await store.list('u1')).length, 1);
    assert.equal((await store.list('u2')).length, 1);
  });

  it('list returns all signals for a user across kinds', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({ user_id: 'u1', kind: 'sender_importance', scope_key: 's1@x', detail: {}, source: 'user_confirmed' });
    await store.upsert({ user_id: 'u1', kind: 'sender_suppressed', scope_key: 's2@x', detail: {}, source: 'user_confirmed' });
    await store.upsert({ user_id: 'u1', kind: 'quietness_preference', scope_key: null, detail: {}, source: 'user_confirmed' });
    const all = await store.list('u1');
    assert.equal(all.length, 3);
    assert.deepEqual(all.map((s) => s.kind).sort(), ['quietness_preference', 'sender_importance', 'sender_suppressed']);
  });

  it('listByKind filters to a single kind', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({ user_id: 'u1', kind: 'sender_importance', scope_key: 's1@x', detail: {}, source: 'user_confirmed' });
    await store.upsert({ user_id: 'u1', kind: 'sender_importance', scope_key: 's2@x', detail: {}, source: 'user_confirmed' });
    await store.upsert({ user_id: 'u1', kind: 'topic_importance', scope_key: 'school', detail: {}, source: 'user_confirmed' });
    const senders = await store.listByKind('u1', 'sender_importance');
    assert.equal(senders.length, 2);
    assert.deepEqual(senders.map((s) => s.scope_key).sort(), ['s1@x', 's2@x']);
  });
});

describe('InMemoryMemorySignalStore — delete', () => {
  it('returns true when removing an existing signal, false when nothing to remove', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({ user_id: 'u1', kind: 'sender_importance', scope_key: 's@x', detail: {}, source: 'user_confirmed' });
    assert.equal(await store.delete('u1', 'sender_importance', 's@x'), true);
    assert.equal(await store.delete('u1', 'sender_importance', 's@x'), false);
    assert.equal(await store.get('u1', 'sender_importance', 's@x'), null);
  });

  it('delete is scoped per (user, kind, scope_key)', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({ user_id: 'u1', kind: 'sender_importance', scope_key: 's1@x', detail: {}, source: 'user_confirmed' });
    await store.upsert({ user_id: 'u1', kind: 'sender_importance', scope_key: 's2@x', detail: {}, source: 'user_confirmed' });
    await store.delete('u1', 'sender_importance', 's1@x');
    assert.equal(await store.get('u1', 'sender_importance', 's1@x'), null);
    assert.ok(await store.get('u1', 'sender_importance', 's2@x'));
  });
});

describe('InMemoryMemorySignalStore — detail redaction', () => {
  it('redacts sensitive keys before persisting', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({
      user_id: 'u1',
      kind: 'sender_importance',
      scope_key: 's@x',
      detail: { access_token: 'plaintext', importance: 'high' },
      source: 'user_confirmed'
    });
    const s = await store.get('u1', 'sender_importance', 's@x');
    const d = s?.detail as Record<string, unknown>;
    assert.equal(d.access_token, '<redacted>');
    assert.equal(d.importance, 'high');
  });
});

describe('InMemoryMemorySignalStore — immutability', () => {
  it('returned signals are frozen', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({ user_id: 'u1', kind: 'sender_importance', scope_key: 's@x', detail: { v: 1 }, source: 'user_confirmed' });
    const s = await store.get('u1', 'sender_importance', 's@x');
    assert.ok(s);
    assert.throws(() => {
      (s as unknown as { confidence: number }).confidence = 0.1;
    });
  });
});
