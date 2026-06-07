// Phase v0.5.12 — rankEmailWithLivePil two-call hybrid tests.
//
// LOAD-BEARING coverage:
//   C1 — Kill switch off → single rankEmail call, no audit payload (BB7 in-vitro)
//   C2 — Kill switch on + canonical PIL row → two calls + audit payload
//   C4 — final_delta = clamp(raw_delta, ±cap); final_score = baseline + final_delta
//   C5 — audit_payload is null when no PIL context applied
//   C11 — Cap not bypassable (BB8 in-vitro: extreme scores still clamp to cap)
//   BB6 in-vitro — non-canonical scope_key (defensive secondary filter)
//
// The wrapper is pure with respect to side effects EXCEPT it makes
// (one or two) model router calls. The router is mocked so behavior is
// deterministic. The audit WRITE itself is a separate function
// (writeBrevioRankPilAppliedAudit) tested independently.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryCostStore } from '../core/cost-tracking.ts';
import { type RawEmailContext } from '../core/egress-policy.ts';
import { type BackendResult, type ModelBackend, createModelRouter } from '../core/model-router.ts';
import { type AuditStore, type AuditEntry } from '../core/audit.ts';

import { rankEmailWithLivePil, writeBrevioRankPilAppliedAudit, modelMentionedPilInReason } from './index.ts';
import { buildRankerPrompt } from './prompt.ts';
import { type PilContext } from './pil-context.ts';

const CANONICAL_HMAC = 'eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee';
const LEGACY_PLACEHOLDER = 'message:19e92fe1ec00b978';

// Minimal AuditStore stub that records writes.
class RecordingAuditStore implements AuditStore {
  public readonly writes: AuditEntry[] = [];
  async write(entry: Omit<AuditEntry, 'id' | 'occurred_at'>): Promise<void> {
    this.writes.push({
      id: this.writes.length + 1,
      occurred_at: new Date().toISOString(),
      ...entry
    } as AuditEntry);
  }
  async recent(_userId: string, _limit?: number): Promise<AuditEntry[]> {
    return this.writes;
  }
}

function fakeRaw(): RawEmailContext {
  return Object.freeze({
    message_id: 'msg-1',
    thread_id: 'thr-1',
    sender_email: 'sarah@school.edu',
    sender_name: 'Sarah Johnson',
    subject: 'Interview form due tonight',
    body_plain: 'Hi, please submit the form.',
    body_html: '<html>...</html>',
    headers: {},
    attachments: [],
    received_at: new Date('2026-05-22T18:30:00.000Z')
  } as RawEmailContext);
}

function freshPilContext(overrides: Partial<PilContext> = {}): PilContext {
  return Object.freeze({
    sender_importance_score: 0.30,
    sender_importance_n_events: 3,
    sender_suppressed: false,
    last_updated: '2026-05-21T00:00:00Z',
    decay_factor_applied: 1.0,
    ...overrides
  });
}

/**
 * Build a router with a backend that returns different scores for baseline
 * vs PIL prompts. We distinguish by the presence of the "PIL prior" marker
 * in the prompt (added by buildPilContextBlock when pil_context is non-null).
 *
 * The backend implements ModelBackend.call({prompt, timeout_ms}) directly so
 * we can branch on prompt content at call time — MockModelBackend's static
 * response map can't do that with dynamic two-call hybrid prompts.
 */
function makeTwoCallRouter(opts: {
  baselineScore: number;
  pilScore: number;
  baselineReason?: string;
  pilReason?: string;
}): { router: ReturnType<typeof createModelRouter>; cost: InMemoryCostStore } {
  const cost = new InMemoryCostStore();
  const router = createModelRouter({ costStore: cost });
  const baselineReason = opts.baselineReason ?? 'baseline reason';
  const pilReason = opts.pilReason ?? 'pil reason';

  const dualBackend: ModelBackend = {
    name(): string {
      return 'mock-dual';
    },
    async call(req: { prompt: string; timeout_ms: number }): Promise<BackendResult> {
      const isPil = req.prompt.includes('PIL prior');
      const text = isPil
        ? `{"label":"important","score":${opts.pilScore},"reason":"${pilReason}"}`
        : `{"label":"important","score":${opts.baselineScore},"reason":"${baselineReason}"}`;
      return {
        text,
        input_tokens: 100,
        output_tokens: 12,
        model_name: 'mock-dual',
        latency_ms: 5
      };
    }
  };
  router.registerBackend('classification', dualBackend);
  return { router, cost };
}

describe('rankEmailWithLivePil — C1 / BB7 kill switch off → single call, no audit', () => {
  it('returns baseline-only result + null audit_payload when pil_live_enabled=false (even if pil_context is non-null)', async () => {
    const { router } = makeTwoCallRouter({ baselineScore: 0.5, pilScore: 0.9 });
    const audit = new RecordingAuditStore();
    const r = await rankEmailWithLivePil(
      {
        raw: fakeRaw(),
        user_id: 'u-1',
        pil_context: freshPilContext({ sender_importance_score: +1.0 }),
        sender_email_hash: CANONICAL_HMAC
      },
      { router, auditStore: audit, pil_live_enabled: false, pil_score_cap: 0.15 }
    );
    assert.equal(r.audit_payload, null);
    assert.equal(r.baseline_result, null);
    assert.equal(r.pil_result, null);
    assert.equal(r.result.ok, true);
    if (r.result.ok) {
      assert.equal(r.result.decision.score, 0.5, 'kill switch off → baseline-only score');
      assert.equal(r.result.prompt_version, 'ranker-v0.2.0');
    }
  });
});

describe('rankEmailWithLivePil — BB6 in-vitro: non-canonical sender_email_hash → single call, no audit', () => {
  it('returns baseline-only result + null audit_payload when sender_email_hash is a legacy message:<id>', async () => {
    const { router } = makeTwoCallRouter({ baselineScore: 0.5, pilScore: 0.9 });
    const audit = new RecordingAuditStore();
    const r = await rankEmailWithLivePil(
      {
        raw: fakeRaw(),
        user_id: 'u-1',
        pil_context: freshPilContext(),
        sender_email_hash: LEGACY_PLACEHOLDER
      },
      { router, auditStore: audit, pil_live_enabled: true, pil_score_cap: 0.15 }
    );
    assert.equal(r.audit_payload, null);
    assert.equal(r.result.ok, true);
    if (r.result.ok) {
      assert.equal(r.result.decision.score, 0.5);
    }
  });

  it('returns baseline-only result when pil_context is null (even if canonical hash + kill switch on)', async () => {
    const { router } = makeTwoCallRouter({ baselineScore: 0.5, pilScore: 0.9 });
    const audit = new RecordingAuditStore();
    const r = await rankEmailWithLivePil(
      {
        raw: fakeRaw(),
        user_id: 'u-1',
        pil_context: null,
        sender_email_hash: CANONICAL_HMAC
      },
      { router, auditStore: audit, pil_live_enabled: true, pil_score_cap: 0.15 }
    );
    assert.equal(r.audit_payload, null);
    assert.equal(r.result.ok, true);
    if (r.result.ok) {
      assert.equal(r.result.decision.score, 0.5);
    }
  });
});

describe('rankEmailWithLivePil — C2 two-call hybrid happy path', () => {
  it('runs baseline + PIL calls, returns clamped final_score, audit_payload populated with 9 fields', async () => {
    // baseline=0.50, pil=0.58 → raw_delta=+0.08, within cap → not capped
    const { router } = makeTwoCallRouter({
      baselineScore: 0.5,
      pilScore: 0.58,
      pilReason: 'counselor — given your past positive history, this looks worth your attention tonight'
    });
    const audit = new RecordingAuditStore();
    const r = await rankEmailWithLivePil(
      {
        raw: fakeRaw(),
        user_id: 'u-1',
        pil_context: freshPilContext({ sender_importance_score: 0.30 }),
        sender_email_hash: CANONICAL_HMAC
      },
      { router, auditStore: audit, pil_live_enabled: true, pil_score_cap: 0.15 }
    );
    assert.equal(r.result.ok, true);
    if (r.result.ok) {
      // final_score = 0.5 + clamp(0.08, ±0.15) = 0.58
      assert.ok(Math.abs(r.result.decision.score - 0.58) < 1e-9);
      assert.equal(r.result.prompt_version, 'ranker-v0.3.0');
    }
    assert.ok(r.audit_payload !== null);
    assert.equal(r.audit_payload!.pil_signal_kinds_present.includes('sender_importance'), true);
    assert.equal(r.audit_payload!.pil_signal_kinds_present.includes('sender_suppressed'), false);
    assert.ok(Math.abs(r.audit_payload!.score_before_pil_cap - 0.58) < 1e-9);
    assert.ok(Math.abs(r.audit_payload!.score_after_pil_cap - 0.58) < 1e-9);
    assert.ok(Math.abs(r.audit_payload!.pil_score_delta - 0.08) < 1e-9);
    assert.equal(r.audit_payload!.pil_score_delta_was_capped, false);
    assert.equal(r.audit_payload!.model_mentioned_pil_in_reason, true);
    assert.equal(r.audit_payload!.source_surface, 'email_alert');
    assert.equal(r.audit_payload!.scope_key_hash, CANONICAL_HMAC);
  });

  it('preserves the PIL call reason text (not the baseline reason) so rank.reason carries the priored decision', async () => {
    const { router } = makeTwoCallRouter({
      baselineScore: 0.5,
      pilScore: 0.55,
      baselineReason: 'baseline-vanilla',
      pilReason: 'pil-aware'
    });
    const audit = new RecordingAuditStore();
    const r = await rankEmailWithLivePil(
      {
        raw: fakeRaw(),
        user_id: 'u-1',
        pil_context: freshPilContext(),
        sender_email_hash: CANONICAL_HMAC
      },
      { router, auditStore: audit, pil_live_enabled: true, pil_score_cap: 0.15 }
    );
    assert.equal(r.result.ok, true);
    if (r.result.ok) {
      assert.equal(r.result.decision.reason, 'pil-aware');
    }
  });
});

describe('rankEmailWithLivePil — C4/C11/BB8 cap is REAL', () => {
  it('clamps positive raw_delta above cap to exactly +cap; was_capped=true', async () => {
    const { router } = makeTwoCallRouter({ baselineScore: 0.40, pilScore: 0.95 });
    const audit = new RecordingAuditStore();
    const r = await rankEmailWithLivePil(
      {
        raw: fakeRaw(),
        user_id: 'u-1',
        pil_context: freshPilContext({ sender_importance_score: +1.0 }),
        sender_email_hash: CANONICAL_HMAC
      },
      { router, auditStore: audit, pil_live_enabled: true, pil_score_cap: 0.15 }
    );
    assert.ok(r.audit_payload !== null);
    // raw_delta = 0.55; clamped = +0.15; was_capped = true
    assert.ok(Math.abs(r.audit_payload!.pil_score_delta - 0.15) < 1e-9);
    assert.equal(r.audit_payload!.pil_score_delta_was_capped, true);
    assert.equal(r.result.ok, true);
    if (r.result.ok) {
      // final = 0.40 + 0.15 = 0.55
      assert.ok(Math.abs(r.result.decision.score - 0.55) < 1e-9);
    }
  });

  it('clamps negative raw_delta below -cap to exactly -cap; was_capped=true', async () => {
    const { router } = makeTwoCallRouter({ baselineScore: 0.90, pilScore: 0.20 });
    const audit = new RecordingAuditStore();
    const r = await rankEmailWithLivePil(
      {
        raw: fakeRaw(),
        user_id: 'u-1',
        pil_context: freshPilContext({ sender_importance_score: -1.0, sender_suppressed: true }),
        sender_email_hash: CANONICAL_HMAC
      },
      { router, auditStore: audit, pil_live_enabled: true, pil_score_cap: 0.15 }
    );
    assert.ok(r.audit_payload !== null);
    // raw_delta = -0.70; clamped = -0.15
    assert.ok(Math.abs(r.audit_payload!.pil_score_delta + 0.15) < 1e-9);
    assert.equal(r.audit_payload!.pil_score_delta_was_capped, true);
    assert.equal(r.result.ok, true);
    if (r.result.ok) {
      // final = 0.90 + (-0.15) = 0.75
      assert.ok(Math.abs(r.result.decision.score - 0.75) < 1e-9);
    }
    // Both signal kinds present in audit
    assert.ok(r.audit_payload!.pil_signal_kinds_present.includes('sender_suppressed'));
  });

  it('respects custom cap value 0.05 (smaller cap → tighter clamp)', async () => {
    const { router } = makeTwoCallRouter({ baselineScore: 0.40, pilScore: 0.60 });
    const audit = new RecordingAuditStore();
    const r = await rankEmailWithLivePil(
      {
        raw: fakeRaw(),
        user_id: 'u-1',
        pil_context: freshPilContext(),
        sender_email_hash: CANONICAL_HMAC
      },
      { router, auditStore: audit, pil_live_enabled: true, pil_score_cap: 0.05 }
    );
    assert.ok(r.audit_payload !== null);
    // raw_delta = +0.20; clamped at +0.05
    assert.ok(Math.abs(r.audit_payload!.pil_score_delta - 0.05) < 1e-9);
    assert.equal(r.audit_payload!.pil_score_delta_was_capped, true);
  });

  it('clamps final_score to [0, 1]', async () => {
    const { router } = makeTwoCallRouter({ baselineScore: 0.97, pilScore: 0.99 });
    const audit = new RecordingAuditStore();
    const r = await rankEmailWithLivePil(
      {
        raw: fakeRaw(),
        user_id: 'u-1',
        pil_context: freshPilContext(),
        sender_email_hash: CANONICAL_HMAC
      },
      { router, auditStore: audit, pil_live_enabled: true, pil_score_cap: 0.15 }
    );
    assert.equal(r.result.ok, true);
    if (r.result.ok) {
      // baseline=0.97; raw_delta=+0.02; clamped=+0.02; final=0.99 (within [0,1])
      assert.ok(r.result.decision.score <= 1);
      assert.ok(r.result.decision.score >= 0);
    }
  });
});

describe('modelMentionedPilInReason — regex transparency floor', () => {
  it('detects clear references to a prior / past behavior', () => {
    assert.equal(modelMentionedPilInReason('despite past corrections, this seems worth seeing'), true);
    assert.equal(modelMentionedPilInReason('counselor — given your prior feedback, still important'), true);
    assert.equal(modelMentionedPilInReason('you usually ignore this sender, but this one has a deadline'), true);
    assert.equal(modelMentionedPilInReason('even though you previously ignored them'), true);
  });
  it('does NOT flag generic reasons that do not reference history', () => {
    assert.equal(modelMentionedPilInReason('Sarah needs your sign-off tonight'), false);
    assert.equal(modelMentionedPilInReason('Weekly LinkedIn jobs digest — nothing personal or time-sensitive.'), false);
  });
  it('handles empty / invalid input safely', () => {
    assert.equal(modelMentionedPilInReason(''), false);
    assert.equal(modelMentionedPilInReason(null as unknown as string), false);
  });
});

describe('writeBrevioRankPilAppliedAudit — Q6.A 9-field audit shape', () => {
  it('writes an audit row with action=brevio.rank.pil_applied and all 9 detail fields', async () => {
    const audit = new RecordingAuditStore();
    await writeBrevioRankPilAppliedAudit(audit, 'founder', 42, {
      pil_signal_kinds_present: ['sender_importance', 'sender_suppressed'],
      score_before_pil_cap: 0.85,
      score_after_pil_cap: 0.65,
      pil_score_delta: -0.15,
      pil_score_delta_was_capped: true,
      model_mentioned_pil_in_reason: true,
      source_surface: 'email_alert',
      scope_key_hash: CANONICAL_HMAC
    });
    assert.equal(audit.writes.length, 1);
    const entry = audit.writes[0]!;
    assert.equal(entry.action, 'brevio.rank.pil_applied');
    assert.equal(entry.actor_user_id, 'founder');
    assert.equal(entry.target, 'rank_result:42');
    assert.equal(entry.result, 'success');
    const detail = entry.detail as Record<string, unknown>;
    assert.equal(detail.rank_result_id, 42);
    assert.deepEqual(detail.pil_signal_kinds_present, ['sender_importance', 'sender_suppressed']);
    assert.equal(detail.score_before_pil_cap, 0.85);
    assert.equal(detail.score_after_pil_cap, 0.65);
    assert.equal(detail.pil_score_delta, -0.15);
    assert.equal(detail.pil_score_delta_was_capped, true);
    assert.equal(detail.model_mentioned_pil_in_reason, true);
    assert.equal(detail.source_surface, 'email_alert');
    assert.equal(detail.scope_key_hash, CANONICAL_HMAC);
    // Only 9 fields total
    const keys = Object.keys(detail).sort();
    assert.deepEqual(keys, [
      'model_mentioned_pil_in_reason',
      'pil_score_delta',
      'pil_score_delta_was_capped',
      'pil_signal_kinds_present',
      'rank_result_id',
      'scope_key_hash',
      'score_after_pil_cap',
      'score_before_pil_cap',
      'source_surface'
    ]);
  });
});
