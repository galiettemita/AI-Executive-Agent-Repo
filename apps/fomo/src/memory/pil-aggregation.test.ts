// Phase v0.5.11 — pil-aggregation unit tests.
//
// These tests are LOAD-BEARING per the founder-locked PASS criteria:
//   C2 — false_positive lowers score, flips suppression at k
//   C7 — one correction does NOT flip
//   C9 — cross-user contamination (DB layer)
//   C15 — brevio.signal.aggregated audit carries all 15 locked detail fields
//   C16 — privacy: no raw email / sender / subject in any detail
//   C18 — reversibility (delete → next aggregation re-creates)

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAuditStore } from '../core/audit.ts';
import {
  applyPilAggregation,
  loadPilTunables,
  PIL_DEFAULT_TUNABLES,
  type PilAggregationDeps,
  type PilAggregationInput
} from './pil-aggregation.ts';
import { InMemoryMemorySignalStore } from './memory-signals.ts';

function buildDeps(): PilAggregationDeps {
  return {
    memoryStore: new InMemoryMemorySignalStore(),
    auditStore: new InMemoryAuditStore()
  };
}

function baseInput(over: Partial<PilAggregationInput> = {}): PilAggregationInput {
  return {
    user_id: 'founder',
    feedback_event_id: 1,
    verb: 'approved',
    dimension: 'importance',
    source_surface: 'email_alert',
    sender_email_hash: 'abcdef1234567890abcdef1234567890',
    k_threshold: 3,
    score_delta: 0.1,
    ...over
  };
}

describe('applyPilAggregation — skip paths', () => {
  it('skips when sender_email_hash is null (no_sender_hash)', async () => {
    const deps = buildDeps();
    const out = await applyPilAggregation(baseInput({ sender_email_hash: null }), deps);
    assert.equal(out.kind, 'skipped');
    if (out.kind === 'skipped') assert.equal(out.reason, 'no_sender_hash');
    assert.equal((await deps.auditStore.recent('founder', 10)).length, 0);
  });

  it('skips when source_surface is not email_alert (inactive_source_surface)', async () => {
    const deps = buildDeps();
    const out = await applyPilAggregation(
      baseInput({ source_surface: 'calendar_reminder' as 'email_alert' }),
      deps
    );
    assert.equal(out.kind, 'skipped');
    if (out.kind === 'skipped') assert.equal(out.reason, 'inactive_source_surface');
  });

  it('skips when verb+dimension does not map to a PIL arm', async () => {
    const deps = buildDeps();
    const out = await applyPilAggregation(
      baseInput({ verb: 'approved', dimension: 'sender' }),
      deps
    );
    assert.equal(out.kind, 'skipped');
    if (out.kind === 'skipped') assert.equal(out.reason, 'inactive_dimension');
  });
});

describe('applyPilAggregation — sender_importance positive arm (this_mattered, more_like_this)', () => {
  it('this_mattered (verb=approved, dimension=importance) → score +δ, n_positive+1, created', async () => {
    const deps = buildDeps();
    const out = await applyPilAggregation(baseInput({ verb: 'approved', dimension: 'importance' }), deps);
    assert.equal(out.kind, 'aggregated');
    if (out.kind !== 'aggregated') throw new Error('unreachable');
    assert.equal(out.memory_signal_kind, 'sender_importance');
    assert.equal(out.memory_signal_action, 'created');
    assert.equal(out.score_before, null);
    assert.equal(out.score_after, 0.1);
    assert.equal(out.score_delta, 0.1);
    assert.equal(out.n_positive_events_after, 1);
    assert.equal(out.n_negative_events_after, 0);
    assert.equal(out.suppression_flipped, false);
  });

  it('more_like_this (verb=approved, dimension=pattern) → score +2δ', async () => {
    const deps = buildDeps();
    const out = await applyPilAggregation(baseInput({ verb: 'approved', dimension: 'pattern' }), deps);
    assert.equal(out.kind, 'aggregated');
    if (out.kind !== 'aggregated') throw new Error('unreachable');
    assert.equal(out.score_after, 0.2);
    assert.equal(out.score_delta, 0.2);
  });

  it('accumulates: 3 consecutive this_mattered → score=0.3, n_positive=3, updated x2', async () => {
    const deps = buildDeps();
    const out1 = await applyPilAggregation(baseInput({ feedback_event_id: 10 }), deps);
    const out2 = await applyPilAggregation(baseInput({ feedback_event_id: 11 }), deps);
    const out3 = await applyPilAggregation(baseInput({ feedback_event_id: 12 }), deps);
    assert.equal(out1.kind === 'aggregated' && out1.memory_signal_action, 'created');
    assert.equal(out2.kind === 'aggregated' && out2.memory_signal_action, 'updated');
    assert.equal(out3.kind === 'aggregated' && out3.memory_signal_action, 'updated');
    if (out3.kind !== 'aggregated') throw new Error('unreachable');
    assert.ok(Math.abs(out3.score_after! - 0.3) < 1e-9, `score_after=${out3.score_after}`);
    assert.equal(out3.n_positive_events_after, 3);
  });

  it('clamps score at +1.0 (no runaway accumulation)', async () => {
    const deps = buildDeps();
    for (let i = 0; i < 15; i++) {
      await applyPilAggregation(baseInput({ feedback_event_id: 100 + i, score_delta: 0.5 }), deps);
    }
    const sig = await deps.memoryStore.get('founder', 'sender_importance', 'abcdef1234567890abcdef1234567890');
    const score = (sig?.detail as { score?: number } | null)?.score;
    assert.equal(score, 1.0);
  });
});

describe('applyPilAggregation — false_positive (one correction does NOT flip; ≥k flips)', () => {
  it('C7 LOAD-BEARING: one false_positive → score -δ, n_negative=1, NO sender_suppressed write', async () => {
    const deps = buildDeps();
    const out = await applyPilAggregation(baseInput({ verb: 'corrected', dimension: 'ranker_label' }), deps);
    if (out.kind !== 'aggregated') throw new Error('unreachable');
    assert.equal(out.memory_signal_kind, 'sender_importance');
    assert.ok(Math.abs(out.score_after! - -0.1) < 1e-9, `score_after=${out.score_after}`);
    assert.equal(out.n_negative_events_after, 1);
    assert.equal(out.suppression_flipped, false);
    const suppressed = await deps.memoryStore.get('founder', 'sender_suppressed', 'abcdef1234567890abcdef1234567890');
    assert.equal(suppressed, null);
  });

  it('C2 LOAD-BEARING: k=3 consecutive false_positives → 3rd write flips sender_suppressed=true', async () => {
    const deps = buildDeps();
    await applyPilAggregation(baseInput({ verb: 'corrected', dimension: 'ranker_label', feedback_event_id: 1 }), deps);
    const r2 = await applyPilAggregation(baseInput({ verb: 'corrected', dimension: 'ranker_label', feedback_event_id: 2 }), deps);
    if (r2.kind !== 'aggregated') throw new Error('unreachable');
    assert.equal(r2.suppression_flipped, false, 'k=2 must NOT flip');
    const r3 = await applyPilAggregation(baseInput({ verb: 'corrected', dimension: 'ranker_label', feedback_event_id: 3 }), deps);
    if (r3.kind !== 'aggregated') throw new Error('unreachable');
    assert.equal(r3.suppression_flipped, true, 'k=3 must flip');
    assert.equal(r3.memory_signal_kind, 'sender_suppressed');
    assert.equal(r3.memory_signal_action, 'created');
    const suppressed = await deps.memoryStore.get('founder', 'sender_suppressed', 'abcdef1234567890abcdef1234567890');
    const det = suppressed?.detail as { suppressed?: boolean; set_by?: string } | null;
    assert.equal(det?.suppressed, true);
    assert.equal(det?.set_by, 'threshold_negative_aggregation');
  });

  it('k+1 false_positive does NOT re-flip (idempotent — no redundant audit)', async () => {
    const deps = buildDeps();
    for (let i = 1; i <= 3; i++) {
      await applyPilAggregation(baseInput({ verb: 'corrected', dimension: 'ranker_label', feedback_event_id: i }), deps);
    }
    const auditsBefore = await deps.auditStore.recent('founder', 100);
    const r4 = await applyPilAggregation(baseInput({ verb: 'corrected', dimension: 'ranker_label', feedback_event_id: 4 }), deps);
    if (r4.kind !== 'aggregated') throw new Error('unreachable');
    // 4th write hits sender_importance, but the sender_suppressed flip is NOT
    // emitted again (the prior suppressed row exists).
    assert.equal(r4.memory_signal_kind, 'sender_importance');
    const auditsAfter = await deps.auditStore.recent('founder', 100);
    // Exactly 1 new audit (the sender_importance update). No 2nd audit for
    // sender_suppressed because the row already existed.
    assert.equal(auditsAfter.length, auditsBefore.length + 1);
  });
});

describe('applyPilAggregation — ignore_sender (explicit single-event flip carve-out per doctrine §9.1)', () => {
  it('ignore_sender (verb=ignored, dimension=sender) → sender_suppressed=true on first event, set_by=explicit', async () => {
    const deps = buildDeps();
    const out = await applyPilAggregation(baseInput({ verb: 'ignored', dimension: 'sender' }), deps);
    if (out.kind !== 'aggregated') throw new Error('unreachable');
    assert.equal(out.memory_signal_kind, 'sender_suppressed');
    assert.equal(out.memory_signal_action, 'created');
    assert.equal(out.suppression_flipped, true);
    const suppressed = await deps.memoryStore.get('founder', 'sender_suppressed', 'abcdef1234567890abcdef1234567890');
    const det = suppressed?.detail as { suppressed?: boolean; set_by?: string } | null;
    assert.equal(det?.suppressed, true);
    assert.equal(det?.set_by, 'explicit_ignore_sender');
  });

  it('second ignore_sender → updated (NOT created), suppression_flipped=false', async () => {
    const deps = buildDeps();
    await applyPilAggregation(baseInput({ verb: 'ignored', dimension: 'sender', feedback_event_id: 1 }), deps);
    const out2 = await applyPilAggregation(baseInput({ verb: 'ignored', dimension: 'sender', feedback_event_id: 2 }), deps);
    if (out2.kind !== 'aggregated') throw new Error('unreachable');
    assert.equal(out2.memory_signal_action, 'updated');
    assert.equal(out2.suppression_flipped, false);
  });

  it('does NOT touch sender_importance (categorical signal, not score-based)', async () => {
    const deps = buildDeps();
    await applyPilAggregation(baseInput({ verb: 'ignored', dimension: 'sender' }), deps);
    const importance = await deps.memoryStore.get('founder', 'sender_importance', 'abcdef1234567890abcdef1234567890');
    assert.equal(importance, null);
  });
});

describe('applyPilAggregation — cross-user contamination (C9 LOAD-BEARING)', () => {
  it('User A signal does NOT bind to User B lookup at the same scope_key', async () => {
    // NOTE: this test exercises the (user_id, kind, scope_key) tuple key
    // shape of MemorySignalStore. In production the scope_key itself encodes
    // user_id via hashSenderKey(); the store's tuple key is the structural
    // backstop. Together they make cross-user leak impossible.
    const deps = buildDeps();
    const scope_userA = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
    await applyPilAggregation(
      baseInput({ user_id: 'userA', sender_email_hash: scope_userA, verb: 'ignored', dimension: 'sender' }),
      deps
    );
    // User B looks up the SAME scope_key — must get null.
    const userBLookup = await deps.memoryStore.get('userB', 'sender_suppressed', scope_userA);
    assert.equal(userBLookup, null);
    // User A's row exists.
    const userALookup = await deps.memoryStore.get('userA', 'sender_suppressed', scope_userA);
    assert.notEqual(userALookup, null);
  });
});

describe('applyPilAggregation — brevio.signal.aggregated audit (C15 LOAD-BEARING)', () => {
  it('every aggregation emits a brevio.signal.aggregated audit with all 15 locked detail fields', async () => {
    const deps = buildDeps();
    await applyPilAggregation(baseInput({ feedback_event_id: 42, verb: 'approved', dimension: 'importance' }), deps);
    const audits = await deps.auditStore.recent('founder', 10);
    const agg = audits.find((a) => a.action === 'brevio.signal.aggregated');
    assert.ok(agg, 'brevio.signal.aggregated audit must fire');
    const d = agg!.detail as Record<string, unknown>;
    for (const f of [
      'verb',
      'dimension',
      'feedback_event_id',
      'source_surface',
      'memory_signal_kind',
      'memory_signal_action',
      'memory_signal_scope_key_hash',
      'score_before',
      'score_after',
      'score_delta',
      'n_positive_events_before',
      'n_positive_events_after',
      'n_negative_events_before',
      'n_negative_events_after',
      'suppression_flipped',
      'threshold_k_in_force'
    ]) {
      assert.ok(f in d, `missing field: ${f}`);
    }
    assert.equal(d.verb, 'approved');
    assert.equal(d.dimension, 'importance');
    assert.equal(d.feedback_event_id, 42);
    assert.equal(d.memory_signal_kind, 'sender_importance');
    assert.equal(d.threshold_k_in_force, 3);
  });

  it('C16 LOAD-BEARING: NO raw sender_email / @gmail.com / subject keywords in audit detail', async () => {
    const deps = buildDeps();
    await applyPilAggregation(baseInput({ verb: 'ignored', dimension: 'sender' }), deps);
    const audits = await deps.auditStore.recent('founder', 10);
    const all = JSON.stringify(audits);
    for (const forbidden of ['@gmail.com', 'noreply', 'Subject:', 'unsubscribe', 'this mattered']) {
      assert.equal(all.includes(forbidden), false, `audit detail contains forbidden substring: ${forbidden}`);
    }
  });

  it('k-threshold flip emits TWO audits (sender_importance update + sender_suppressed create)', async () => {
    const deps = buildDeps();
    await applyPilAggregation(baseInput({ verb: 'corrected', dimension: 'ranker_label', feedback_event_id: 1 }), deps);
    await applyPilAggregation(baseInput({ verb: 'corrected', dimension: 'ranker_label', feedback_event_id: 2 }), deps);
    await applyPilAggregation(baseInput({ verb: 'corrected', dimension: 'ranker_label', feedback_event_id: 3 }), deps);
    const audits = await deps.auditStore.recent('founder', 100);
    const agg = audits.filter((a) => a.action === 'brevio.signal.aggregated');
    // 3 events → 3 sender_importance audits + 1 sender_suppressed audit
    // (only the 3rd event flips). recent() returns newest-first.
    assert.equal(agg.length, 4);
    assert.equal((agg[0]!.detail as { memory_signal_kind: string }).memory_signal_kind, 'sender_suppressed');
    assert.equal((agg[0]!.detail as { suppression_flipped: boolean }).suppression_flipped, true);
    assert.equal((agg[1]!.detail as { memory_signal_kind: string }).memory_signal_kind, 'sender_importance');
  });
});

describe('applyPilAggregation — reversibility (C18 LOAD-BEARING)', () => {
  it('delete memory_signal → next aggregation event creates a fresh row', async () => {
    const deps = buildDeps();
    await applyPilAggregation(baseInput({ verb: 'ignored', dimension: 'sender', feedback_event_id: 1 }), deps);
    await deps.memoryStore.delete('founder', 'sender_suppressed', 'abcdef1234567890abcdef1234567890');
    const out2 = await applyPilAggregation(
      baseInput({ verb: 'ignored', dimension: 'sender', feedback_event_id: 2 }),
      deps
    );
    if (out2.kind !== 'aggregated') throw new Error('unreachable');
    assert.equal(out2.memory_signal_action, 'created');
    assert.equal(out2.suppression_flipped, true);
  });
});

describe('loadPilTunables — Q5.C env parsing + bounds', () => {
  it('returns defaults when no env vars set', () => {
    const t = loadPilTunables({});
    assert.deepEqual(t, PIL_DEFAULT_TUNABLES);
  });

  it('parses valid overrides', () => {
    const t = loadPilTunables({
      FOMO_PIL_K_THRESHOLD: '5',
      FOMO_PIL_SCORE_DELTA: '0.25',
      FOMO_PIL_RECENCY_FULL_DAYS: '30',
      FOMO_PIL_RECENCY_DECAY_DAYS: '60'
    });
    assert.equal(t.k_threshold, 5);
    assert.equal(t.score_delta, 0.25);
    assert.equal(t.recency_full_days, 30);
    assert.equal(t.recency_decay_days, 60);
  });

  it('throws on out-of-bounds k_threshold', () => {
    assert.throws(() => loadPilTunables({ FOMO_PIL_K_THRESHOLD: '0' }));
    assert.throws(() => loadPilTunables({ FOMO_PIL_K_THRESHOLD: '-3' }));
    assert.throws(() => loadPilTunables({ FOMO_PIL_K_THRESHOLD: 'abc' }));
    assert.throws(() => loadPilTunables({ FOMO_PIL_K_THRESHOLD: '1.5' }));
  });

  it('throws on out-of-bounds score_delta (must be in (0, 0.5])', () => {
    assert.throws(() => loadPilTunables({ FOMO_PIL_SCORE_DELTA: '0' }));
    assert.throws(() => loadPilTunables({ FOMO_PIL_SCORE_DELTA: '-0.1' }));
    assert.throws(() => loadPilTunables({ FOMO_PIL_SCORE_DELTA: '0.6' }));
    assert.throws(() => loadPilTunables({ FOMO_PIL_SCORE_DELTA: '999' }));
  });

  it('allows recency_decay_days=0 (hard cliff)', () => {
    const t = loadPilTunables({ FOMO_PIL_RECENCY_DECAY_DAYS: '0' });
    assert.equal(t.recency_decay_days, 0);
  });
});
