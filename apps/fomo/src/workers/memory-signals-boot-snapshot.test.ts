// Phase 3G.1 item #10 — regression test for memory_signals boot snapshot.
//
// Original incident captured: 2026-05-29 01:00 UTC during the 3G
// smoke setup. memory_signals.stop_active=true from the previous
// day's 3F.2 smoke survived silently into the next day. The next
// alert was correctly blocked by stop_enforced — but the founder had
// no boot-time signal that stop_active was still set. Discovery cost
// ~10 min of manual psql querying. The snapshot at boot makes that
// state loud.
//
// fail-before/pass-after: this helper file didn't exist before this
// PR. The test file imports it; if the function isn't there the
// suite fails at compile time. With the helper landed and the boot
// wiring calling it, the test asserts the named-safe snapshot shape.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryMemorySignalStore } from '../memory/memory-signals.js';
import { snapshotMemorySignalsForBoot } from './memory-signals-boot-snapshot.js';

describe('snapshotMemorySignalsForBoot (Phase 3G.1 item #10)', () => {
  it('returns empty when the user has no memory signals', async () => {
    const store = new InMemoryMemorySignalStore();
    const snap = await snapshotMemorySignalsForBoot('founder', { memoryStore: store });
    assert.deepEqual(snap, []);
  });

  it('surfaces stop_active=true with named-safe fields only (incident reproduction)', async () => {
    const store = new InMemoryMemorySignalStore();
    // Pin a fake "yesterday" timestamp so the age calculation is deterministic.
    const yesterday = new Date('2026-05-28T01:12:21.720Z');
    await store.upsert({
      user_id: 'founder',
      kind: 'stop_active',
      scope_key: null,
      detail: { active: true, recorded_at: yesterday.toISOString() },
      source: 'user_confirmed',
      confidence: 1.0,
      updated_at: yesterday.toISOString()
    });

    // "Now" = 18 hours after yesterday.
    const nowMs = yesterday.getTime() + 18 * 60 * 60 * 1000;
    const snap = await snapshotMemorySignalsForBoot(
      'founder',
      { memoryStore: store },
      { now: () => nowMs }
    );

    assert.equal(snap.length, 1);
    const entry = snap[0];
    assert.equal(entry.user_id, 'founder');
    assert.equal(entry.kind, 'stop_active');
    assert.equal(entry.scope_key, null);
    assert.equal(entry.source, 'user_confirmed');
    assert.equal(entry.confidence, 1.0);
    assert.equal(entry.active_flag, true);
    // 18 hours in seconds.
    assert.equal(entry.age_seconds, 18 * 60 * 60);
  });

  it('does NOT include the raw detail body in the snapshot (privacy invariant)', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({
      user_id: 'founder',
      kind: 'stop_active',
      scope_key: null,
      detail: {
        active: true,
        recorded_at: '2026-05-28T01:12:21.720Z',
        // Plant a canary value that MUST NOT appear in the snapshot.
        sensitive_detail_canary: 'DO-NOT-LEAK-INTO-BOOT-SNAPSHOT-12345'
      },
      source: 'user_confirmed',
      confidence: 1.0
    });
    const snap = await snapshotMemorySignalsForBoot('founder', { memoryStore: store });
    const dump = JSON.stringify(snap);
    assert.equal(
      dump.includes('DO-NOT-LEAK-INTO-BOOT-SNAPSHOT-12345'),
      false,
      'snapshot leaked raw detail content'
    );
    assert.equal(dump.includes('recorded_at'), false, 'snapshot leaked detail key');
  });

  it('prunes signals below the confidence threshold (default 0.5)', async () => {
    const store = new InMemoryMemorySignalStore();
    // High-confidence stop signal — should appear.
    await store.upsert({
      user_id: 'founder',
      kind: 'stop_active',
      scope_key: null,
      detail: { active: false },
      source: 'user_confirmed',
      confidence: 1.0
    });
    // Low-confidence inferred signal — should be pruned.
    await store.upsert({
      user_id: 'founder',
      kind: 'sender_importance',
      scope_key: 'someone@example.com',
      detail: { score: 0.3 },
      source: 'inferred',
      confidence: 0.3
    });
    const snap = await snapshotMemorySignalsForBoot('founder', { memoryStore: store });
    assert.equal(snap.length, 1);
    assert.equal(snap[0].kind, 'stop_active');
  });

  it('respects a caller-overridden confidence threshold', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({
      user_id: 'founder',
      kind: 'sender_importance',
      scope_key: 'someone@example.com',
      detail: { score: 0.6 },
      source: 'inferred',
      confidence: 0.6
    });
    // Threshold above 0.6 — empty.
    const empty = await snapshotMemorySignalsForBoot(
      'founder',
      { memoryStore: store },
      { minConfidence: 0.7 }
    );
    assert.deepEqual(empty, []);
    // Threshold at 0.5 — one entry.
    const filled = await snapshotMemorySignalsForBoot(
      'founder',
      { memoryStore: store },
      { minConfidence: 0.5 }
    );
    assert.equal(filled.length, 1);
  });

  it('orders entries newest-first so the most recent change appears at the top', async () => {
    const store = new InMemoryMemorySignalStore();
    const older = new Date('2026-05-27T00:00:00.000Z');
    const newer = new Date('2026-05-28T00:00:00.000Z');
    await store.upsert({
      user_id: 'founder',
      kind: 'sender_importance',
      scope_key: 'older@example.com',
      detail: { score: 0.9 },
      source: 'feedback_derived',
      confidence: 0.9,
      updated_at: older.toISOString()
    });
    await store.upsert({
      user_id: 'founder',
      kind: 'sender_importance',
      scope_key: 'newer@example.com',
      detail: { score: 0.9 },
      source: 'feedback_derived',
      confidence: 0.9,
      updated_at: newer.toISOString()
    });
    const snap = await snapshotMemorySignalsForBoot(
      'founder',
      { memoryStore: store },
      { now: () => new Date('2026-05-29T00:00:00.000Z').getTime() }
    );
    assert.equal(snap.length, 2);
    assert.equal(snap[0].scope_key, 'newer@example.com'); // smaller age first
    assert.equal(snap[1].scope_key, 'older@example.com');
  });

  it('active_flag is null for kinds that do NOT carry active/inactive semantics', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({
      user_id: 'founder',
      kind: 'sender_importance',
      scope_key: 'someone@example.com',
      detail: { score: 0.8 },
      source: 'feedback_derived',
      confidence: 0.8
    });
    const snap = await snapshotMemorySignalsForBoot('founder', { memoryStore: store });
    assert.equal(snap.length, 1);
    assert.equal(snap[0].active_flag, null);
  });

  it('return value and each entry are frozen', async () => {
    const store = new InMemoryMemorySignalStore();
    await store.upsert({
      user_id: 'founder',
      kind: 'stop_active',
      scope_key: null,
      detail: { active: true },
      source: 'user_confirmed',
      confidence: 1.0
    });
    const snap = await snapshotMemorySignalsForBoot('founder', { memoryStore: store });
    assert.ok(Object.isFrozen(snap));
    assert.ok(Object.isFrozen(snap[0]));
  });
});
