import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryCostStore } from '../core/cost-tracking.ts';
import { type RawEmailContext } from '../core/egress-policy.ts';
import { MockModelBackend, normalizePrompt } from '../core/model-backends/mock.ts';
import { createModelRouter } from '../core/model-router.ts';

import { PROMPT_VERSION, rankEmail } from './index.ts';
import { buildRankerPrompt } from './prompt.ts';

// Synthesize a RawEmailContext that mirrors what GmailClient.projectGmailMessage
// produces. body_html / headers / attachments deliberately carry leak
// canaries so we can verify nothing forbidden reaches the model prompt.
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
      Received: 'LEAK-CANARY-RECEIVED',
      'X-Probe-Header': 'LEAK-CANARY-XPROBE'
    },
    attachments: [{ filename: 'LEAK-CANARY-form.pdf', size_bytes: 1234 }],
    received_at: new Date('2026-05-22T18:30:00.000Z'),
    ...overrides
  } as RawEmailContext);
}

function makeRouter() {
  const cost = new InMemoryCostStore();
  const router = createModelRouter({ costStore: cost });
  return { router, cost };
}

describe('rankEmail — happy path (mock backend)', () => {
  it('returns a parsed RankDecision and includes prompt_version + cost', async () => {
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
    });

    const { router, cost } = makeRouter();
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

    const result = await rankEmail({ raw, user_id: 'u-1' }, { router });

    assert.equal(result.ok, true);
    if (result.ok) {
      assert.equal(result.decision.label, 'important');
      assert.equal(result.decision.score, 0.91);
      assert.equal(result.prompt_version, PROMPT_VERSION);
      assert.equal(result.model_name, 'mock-classifier-tiny');
      assert.equal(result.input_tokens, 100);
      assert.equal(result.output_tokens, 12);
      assert.ok(result.estimated_cost_usd > 0, 'expected non-zero cost');
    }

    // Cost record was written through the router.
    const records = await cost.recent('u-1');
    assert.equal(records.length, 1);
    assert.equal(records[0]?.prompt_version, PROMPT_VERSION);
    assert.equal(records[0]?.capability, 'classification');
    assert.equal(records[0]?.schema_valid, true);
  });
});

describe('rankEmail — egress invariant', () => {
  it('prompt never includes body_html, raw header values, or attachment filenames', async () => {
    const raw = fakeRaw();
    let receivedPrompt = '';
    const cost = new InMemoryCostStore();
    const router = createModelRouter({ costStore: cost });

    // Custom backend that captures the prompt for inspection.
    router.registerBackend('classification', {
      name: () => 'capture-backend',
      call: async (req) => {
        receivedPrompt = req.prompt;
        return {
          text: '{"label":"not_important","score":0.1,"reason":"x"}',
          input_tokens: 1,
          output_tokens: 1,
          model_name: 'capture-backend',
          latency_ms: 1
        };
      }
    });

    await rankEmail({ raw, user_id: 'u-1' }, { router });

    // The leak canaries from fakeRaw() must NOT appear anywhere in the
    // prompt that reached the backend.
    assert.equal(receivedPrompt.includes('LEAK-CANARY-BODY-HTML'), false);
    assert.equal(receivedPrompt.includes('LEAK-CANARY-RECEIVED'), false);
    assert.equal(receivedPrompt.includes('LEAK-CANARY-XPROBE'), false);
    assert.equal(receivedPrompt.includes('LEAK-CANARY-form.pdf'), false);
    // Sanity check: things that ARE allowed do appear.
    assert.ok(receivedPrompt.includes('Interview form due tonight'));
    assert.ok(receivedPrompt.includes('sarah@school.edu'));
  });
});

describe('rankEmail — fail-closed paths surface as { ok: false }', () => {
  it('returns no_backend_for_capability when nothing is registered', async () => {
    const cost = new InMemoryCostStore();
    const router = createModelRouter({ costStore: cost });
    const result = await rankEmail({ raw: fakeRaw(), user_id: 'u-1' }, { router });
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'no_backend_for_capability');
      assert.equal(result.model_name, null);
      assert.equal(result.prompt_version, PROMPT_VERSION);
    }
  });

  it('returns backend_error when the backend throws', async () => {
    const cost = new InMemoryCostStore();
    const router = createModelRouter({ costStore: cost });
    router.registerBackend('classification', {
      name: () => 'throwing-backend',
      call: async () => {
        throw new Error('upstream is down');
      }
    });
    const result = await rankEmail({ raw: fakeRaw(), user_id: 'u-1' }, { router });
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'backend_error');
      assert.match(result.reason, /upstream is down/);
    }
  });

  it('returns schema_invalid when the backend returns malformed JSON', async () => {
    const cost = new InMemoryCostStore();
    const router = createModelRouter({ costStore: cost });
    router.registerBackend('classification', {
      name: () => 'broken-backend',
      call: async () => ({
        text: 'I think this email is important.',
        input_tokens: 10,
        output_tokens: 8,
        model_name: 'broken-backend',
        latency_ms: 1
      })
    });
    const result = await rankEmail({ raw: fakeRaw(), user_id: 'u-1' }, { router });
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'schema_invalid');
      assert.equal(result.model_name, 'broken-backend');
    }
  });
});

describe('rankEmail — returned result objects are frozen', () => {
  it('success result is frozen', async () => {
    const raw = fakeRaw();
    const cost = new InMemoryCostStore();
    const router = createModelRouter({ costStore: cost });
    router.registerBackend('classification', {
      name: () => 'b',
      call: async () => ({
        text: '{"label":"important","score":0.5,"reason":"x"}',
        input_tokens: 1,
        output_tokens: 1,
        model_name: 'b',
        latency_ms: 1
      })
    });
    const result = await rankEmail({ raw, user_id: 'u-1' }, { router });
    assert.equal(result.ok, true);
    assert.throws(() => {
      (result as unknown as { ok: boolean }).ok = false;
    });
  });

  it('failure result is frozen', async () => {
    const cost = new InMemoryCostStore();
    const router = createModelRouter({ costStore: cost });
    const result = await rankEmail({ raw: fakeRaw(), user_id: 'u-1' }, { router });
    assert.equal(result.ok, false);
    assert.throws(() => {
      (result as unknown as { ok: boolean }).ok = true;
    });
  });
});
