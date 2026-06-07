// Phase v0.5.12 — prompt assembly + rankEmail PIL-context threading tests.
//
// LOAD-BEARING coverage:
//   C2 — pil_context block included when pilContext is non-null
//   C6 — privacy canary: prompt PIL block has ONLY the 3 allowed structural
//        fields (no raw sender_email, subject, body, snippet, headers)
//   C13 — PROMPT_VERSION baseline call stays 'ranker-v0.2.0';
//         PIL-context call uses 'ranker-v0.3.0'
//
// Coverage gap reminder: the kill-switch contract (FOMO_PIL_LIVE_ENABLED=false
// → bit-identical v0.5.11) is enforced at the worker call site, not inside
// rankEmail. The pil-live-hybrid.test.ts file covers BB7.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryCostStore } from '../core/cost-tracking.ts';
import { type RawEmailContext } from '../core/egress-policy.ts';
import { MockModelBackend, normalizePrompt } from '../core/model-backends/mock.ts';
import { createModelRouter } from '../core/model-router.ts';

import { PROMPT_VERSION, PROMPT_VERSION_WITH_PIL, rankEmail } from './index.ts';
import { buildPilContextBlock, buildRankerPrompt } from './prompt.ts';
import { type PilContext } from './pil-context.ts';

function fakeRaw(overrides: Partial<RawEmailContext> = {}): RawEmailContext {
  return Object.freeze({
    message_id: 'msg-1',
    thread_id: 'thr-1',
    sender_email: 'sarah@school.edu',
    sender_name: 'Sarah Johnson',
    subject: 'Interview form due tonight',
    body_plain: 'Hi Albert, please submit the interview form by 9pm.',
    body_html: '<html><b>LEAK-CANARY-BODY-HTML</b></html>',
    headers: {
      Received: 'LEAK-CANARY-RECEIVED'
    },
    attachments: [{ filename: 'LEAK-CANARY-form.pdf', size_bytes: 1234 }],
    received_at: new Date('2026-05-22T18:30:00.000Z'),
    ...overrides
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

describe('buildPilContextBlock — privacy + voice', () => {
  it('contains the 3 allowed structural fields and nothing else (no PII shape)', () => {
    const block = buildPilContextBlock(freshPilContext());
    assert.ok(block.includes('sender_importance_score'));
    assert.ok(block.includes('sender_suppressed'));
    assert.ok(block.includes('signal_age_days'));
    // Privacy canary — these MUST NOT appear in the block text
    for (const forbidden of [
      'sarah@school.edu',
      'Sarah Johnson',
      'Interview form',
      'Hi Albert',
      '<html',
      'Received:',
      'X-',
      'LEAK-CANARY'
    ]) {
      assert.ok(
        !block.includes(forbidden),
        `forbidden substring "${forbidden}" leaked into PIL block`
      );
    }
  });

  it('frames PIL as a PRIOR not a directive (Q3.A — model can override)', () => {
    const block = buildPilContextBlock(freshPilContext());
    const lower = block.toLowerCase();
    assert.ok(lower.includes('prior') || lower.includes("user's past feedback"));
    assert.ok(lower.includes('may override') || lower.includes('weight the prior modestly'));
  });

  it('signal_age_days computes a non-negative integer from PilContext.last_updated', () => {
    const block = buildPilContextBlock(freshPilContext({ last_updated: '1970-01-01T00:00:00Z' }));
    const match = /signal_age_days: (\d+)/.exec(block);
    assert.ok(match);
    assert.ok(Number(match![1]) > 1000, 'expected very-old basis to produce large age');
  });

  it('signal_age_days is 0 when last_updated is null', () => {
    const block = buildPilContextBlock(freshPilContext({ last_updated: null }));
    assert.ok(block.includes('signal_age_days: 0'));
  });

  it('sender_importance_score is formatted with 3 decimal places (no raw 0.30000000000000004)', () => {
    const block = buildPilContextBlock(freshPilContext({ sender_importance_score: 0.30000000000000004 }));
    assert.ok(block.includes('sender_importance_score: 0.300'));
    assert.ok(!block.includes('0.30000000000000004'));
  });
});

describe('buildRankerPrompt — PIL block inclusion', () => {
  it('does NOT include the PIL block when pilContext is null (baseline call shape)', () => {
    const view = {
      purpose: 'model_ranker' as const,
      sender_email: 'sarah@school.edu',
      sender_name: 'Sarah',
      subject: 'Form',
      body_snippet: 'snip',
      received_at: '2026-05-22T18:30:00.000Z',
      has_attachments: false,
      attachment_count: 0,
      message_id: 'msg-1',
      thread_id: 'thr-1'
    };
    const baseline = buildRankerPrompt(view, null);
    assert.ok(!baseline.includes('PIL prior'));
    assert.ok(!baseline.includes('sender_importance_score'));
    assert.ok(!baseline.includes('sender_suppressed'));
  });

  it('includes the PIL block when pilContext is non-null', () => {
    const view = {
      purpose: 'model_ranker' as const,
      sender_email: 'sarah@school.edu',
      sender_name: 'Sarah',
      subject: 'Form',
      body_snippet: 'snip',
      received_at: '2026-05-22T18:30:00.000Z',
      has_attachments: false,
      attachment_count: 0,
      message_id: 'msg-1',
      thread_id: 'thr-1'
    };
    const withPil = buildRankerPrompt(view, freshPilContext());
    assert.ok(withPil.includes('PIL prior'));
    assert.ok(withPil.includes('sender_importance_score:'));
    assert.ok(withPil.includes('sender_suppressed:'));
    assert.ok(withPil.includes('signal_age_days:'));
  });

  it('PIL block appears BEFORE the email-to-classify section (so the model reads the prior before the email content)', () => {
    const view = {
      purpose: 'model_ranker' as const,
      sender_email: 'sarah@school.edu',
      sender_name: 'Sarah',
      subject: 'Form',
      body_snippet: 'snip',
      received_at: '2026-05-22T18:30:00.000Z',
      has_attachments: false,
      attachment_count: 0,
      message_id: 'msg-1',
      thread_id: 'thr-1'
    };
    const withPil = buildRankerPrompt(view, freshPilContext());
    const pilIdx = withPil.indexOf('PIL prior');
    const emailIdx = withPil.indexOf('Email to classify');
    assert.ok(pilIdx > 0 && emailIdx > 0);
    assert.ok(pilIdx < emailIdx, 'PIL block must appear before email content');
  });
});

describe('rankEmail — pil_context threading', () => {
  it('baseline (pil_context=null) call records prompt_version=ranker-v0.2.0', async () => {
    const raw = fakeRaw();
    const view = buildRankerPrompt({
      purpose: 'model_ranker',
      sender_email: raw.sender_email,
      sender_name: raw.sender_name,
      subject: raw.subject,
      body_snippet: 'Hi Albert, please submit the interview form by 9pm.',
      received_at: raw.received_at.toISOString(),
      has_attachments: true,
      attachment_count: 1,
      message_id: raw.message_id,
      thread_id: raw.thread_id
    }, null);

    const cost = new InMemoryCostStore();
    const router = createModelRouter({ costStore: cost });
    const backend = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: {
        [normalizePrompt(view)]: {
          text: '{"label":"important","score":0.91,"reason":"counselor / time-sensitive"}',
          input_tokens: 100,
          output_tokens: 12,
          latency_ms: 5
        }
      }
    });
    router.registerBackend('classification', backend);

    const result = await rankEmail({ raw, user_id: 'u-1', pil_context: null }, { router });
    assert.equal(result.ok, true);
    if (result.ok) {
      assert.equal(result.prompt_version, PROMPT_VERSION);
      assert.equal(result.prompt_version, 'ranker-v0.2.0');
    }
    const records = await cost.recent('u-1');
    assert.equal(records[0]?.prompt_version, 'ranker-v0.2.0');
  });

  it('pil_context non-null call records prompt_version=ranker-v0.3.0', async () => {
    const raw = fakeRaw();
    const pil = freshPilContext({ sender_importance_score: -0.2 });
    const viewWithPil = buildRankerPrompt({
      purpose: 'model_ranker',
      sender_email: raw.sender_email,
      sender_name: raw.sender_name,
      subject: raw.subject,
      body_snippet: 'Hi Albert, please submit the interview form by 9pm.',
      received_at: raw.received_at.toISOString(),
      has_attachments: true,
      attachment_count: 1,
      message_id: raw.message_id,
      thread_id: raw.thread_id
    }, pil);

    const cost = new InMemoryCostStore();
    const router = createModelRouter({ costStore: cost });
    const backend = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: {
        [normalizePrompt(viewWithPil)]: {
          text: '{"label":"important","score":0.86,"reason":"counselor — despite prior corrections, the interview deadline is tonight"}',
          input_tokens: 110,
          output_tokens: 18,
          latency_ms: 8
        }
      }
    });
    router.registerBackend('classification', backend);

    const result = await rankEmail({ raw, user_id: 'u-1', pil_context: pil }, { router });
    assert.equal(result.ok, true);
    if (result.ok) {
      assert.equal(result.prompt_version, PROMPT_VERSION_WITH_PIL);
      assert.equal(result.prompt_version, 'ranker-v0.3.0');
    }
    const records = await cost.recent('u-1');
    assert.equal(records[0]?.prompt_version, 'ranker-v0.3.0');
  });
});
