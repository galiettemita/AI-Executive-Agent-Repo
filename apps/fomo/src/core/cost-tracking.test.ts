import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  CAPABILITY_TAGS,
  InMemoryCostStore,
  MODEL_PRICING,
  computeEstimatedCost,
  isCapabilityTag
} from './cost-tracking.ts';

describe('CAPABILITY_TAGS', () => {
  it('v0.1 declares exactly one capability tag: classification', () => {
    assert.equal(CAPABILITY_TAGS.length, 1);
    assert.deepEqual([...CAPABILITY_TAGS], ['classification']);
  });

  it('is frozen', () => {
    assert.throws(() => {
      (CAPABILITY_TAGS as unknown as string[]).push('planning');
    });
  });
});

describe('isCapabilityTag', () => {
  it('accepts classification, rejects everything else', () => {
    assert.equal(isCapabilityTag('classification'), true);
    assert.equal(isCapabilityTag('planning'), false);
    assert.equal(isCapabilityTag('extraction'), false);
    assert.equal(isCapabilityTag(''), false);
    assert.equal(isCapabilityTag(null), false);
    assert.equal(isCapabilityTag(42), false);
  });
});

describe('MODEL_PRICING', () => {
  it('exposes the two mock models the Phase 2D backend uses', () => {
    assert.ok('mock-classifier-tiny' in MODEL_PRICING);
    assert.ok('mock-classifier-small' in MODEL_PRICING);
  });

  it('is frozen at the top level and per entry', () => {
    assert.throws(() => {
      (MODEL_PRICING as unknown as Record<string, unknown>)['mystery'] = 'x';
    });
    assert.throws(() => {
      (MODEL_PRICING['mock-classifier-tiny'] as unknown as { input_per_1m_usd: number }).input_per_1m_usd = 0;
    });
  });

  it('small model is more expensive than tiny on both input and output', () => {
    assert.ok(MODEL_PRICING['mock-classifier-small']!.input_per_1m_usd > MODEL_PRICING['mock-classifier-tiny']!.input_per_1m_usd);
    assert.ok(MODEL_PRICING['mock-classifier-small']!.output_per_1m_usd > MODEL_PRICING['mock-classifier-tiny']!.output_per_1m_usd);
  });
});

describe('computeEstimatedCost', () => {
  it('returns 0 for unknown model', () => {
    assert.equal(computeEstimatedCost('mystery-model', 1000, 500), 0);
  });

  it('computes (input/1M * input_price) + (output/1M * output_price)', () => {
    // tiny: input $0.10/1M, output $0.40/1M
    // 1,000,000 input + 1,000,000 output → 0.10 + 0.40 = 0.50
    const got = computeEstimatedCost('mock-classifier-tiny', 1_000_000, 1_000_000);
    assert.ok(Math.abs(got - 0.5) < 1e-9, `got ${got}`);
  });

  it('scales linearly with token count', () => {
    const a = computeEstimatedCost('mock-classifier-tiny', 100_000, 50_000);
    const b = computeEstimatedCost('mock-classifier-tiny', 200_000, 100_000);
    assert.ok(Math.abs(b - 2 * a) < 1e-9);
  });

  it('handles zero tokens', () => {
    assert.equal(computeEstimatedCost('mock-classifier-tiny', 0, 0), 0);
  });
});

describe('InMemoryCostStore', () => {
  it('write + recent round-trip, newest first', async () => {
    const store = new InMemoryCostStore();
    await store.write({
      user_id: 'u1',
      capability: 'classification',
      model_name: 'mock-classifier-tiny',
      prompt_version: 'p1',
      latency_ms: 120,
      input_tokens: 500,
      output_tokens: 50,
      estimated_cost_usd: 0.000_07,
      schema_valid: true
    });
    await store.write({
      user_id: 'u1',
      capability: 'classification',
      model_name: 'mock-classifier-small',
      prompt_version: 'p1',
      latency_ms: 350,
      input_tokens: 800,
      output_tokens: 120,
      estimated_cost_usd: 0.000_38,
      schema_valid: false
    });
    const out = await store.recent('u1');
    assert.equal(out.length, 2);
    assert.equal(out[0]?.model_name, 'mock-classifier-small');
    assert.equal(out[1]?.model_name, 'mock-classifier-tiny');
    assert.equal(out[0]?.schema_valid, false);
  });

  it('isolates records per user', async () => {
    const store = new InMemoryCostStore();
    await store.write({
      user_id: 'u1', capability: 'classification', model_name: 'mock-classifier-tiny',
      prompt_version: 'p1', latency_ms: 1, input_tokens: 1, output_tokens: 1,
      estimated_cost_usd: 0.1, schema_valid: true
    });
    await store.write({
      user_id: 'u2', capability: 'classification', model_name: 'mock-classifier-tiny',
      prompt_version: 'p1', latency_ms: 1, input_tokens: 1, output_tokens: 1,
      estimated_cost_usd: 0.2, schema_valid: true
    });
    assert.equal((await store.recent('u1')).length, 1);
    assert.equal((await store.recent('u2')).length, 1);
  });

  it('sumByModel totals estimated_cost_usd across calls for that user+model', async () => {
    const store = new InMemoryCostStore();
    await store.write({
      user_id: 'u1', capability: 'classification', model_name: 'mock-classifier-tiny',
      prompt_version: 'p1', latency_ms: 1, input_tokens: 1, output_tokens: 1,
      estimated_cost_usd: 0.10, schema_valid: true
    });
    await store.write({
      user_id: 'u1', capability: 'classification', model_name: 'mock-classifier-tiny',
      prompt_version: 'p1', latency_ms: 1, input_tokens: 1, output_tokens: 1,
      estimated_cost_usd: 0.25, schema_valid: true
    });
    await store.write({
      user_id: 'u1', capability: 'classification', model_name: 'mock-classifier-small',
      prompt_version: 'p1', latency_ms: 1, input_tokens: 1, output_tokens: 1,
      estimated_cost_usd: 0.99, schema_valid: true
    });
    assert.ok(Math.abs((await store.sumByModel('u1', 'mock-classifier-tiny')) - 0.35) < 1e-9);
    assert.ok(Math.abs((await store.sumByModel('u1', 'mock-classifier-small')) - 0.99) < 1e-9);
    assert.equal(await store.sumByModel('u1', 'mock-classifier-medium'), 0);
    assert.equal(await store.sumByModel('u2', 'mock-classifier-tiny'), 0);
  });

  it('sumByPeriod filters by occurred_at lexicographic ISO range', async () => {
    const store = new InMemoryCostStore();
    await store.write({
      user_id: 'u1', capability: 'classification', model_name: 'mock-classifier-tiny',
      prompt_version: 'p1', latency_ms: 1, input_tokens: 1, output_tokens: 1,
      estimated_cost_usd: 0.10, schema_valid: true,
      occurred_at: '2026-05-01T00:00:00.000Z'
    });
    await store.write({
      user_id: 'u1', capability: 'classification', model_name: 'mock-classifier-tiny',
      prompt_version: 'p1', latency_ms: 1, input_tokens: 1, output_tokens: 1,
      estimated_cost_usd: 0.20, schema_valid: true,
      occurred_at: '2026-05-15T00:00:00.000Z'
    });
    await store.write({
      user_id: 'u1', capability: 'classification', model_name: 'mock-classifier-tiny',
      prompt_version: 'p1', latency_ms: 1, input_tokens: 1, output_tokens: 1,
      estimated_cost_usd: 0.30, schema_valid: true,
      occurred_at: '2026-06-01T00:00:00.000Z'
    });
    const may = await store.sumByPeriod('u1', '2026-05-01T00:00:00.000Z', '2026-05-31T23:59:59.999Z');
    assert.ok(Math.abs(may - 0.30) < 1e-9);
    const june = await store.sumByPeriod('u1', '2026-06-01T00:00:00.000Z', '2026-06-30T23:59:59.999Z');
    assert.ok(Math.abs(june - 0.30) < 1e-9);
  });

  it('schema_valid=false records are still counted in cost sums (wasted spend is real spend)', async () => {
    const store = new InMemoryCostStore();
    await store.write({
      user_id: 'u1', capability: 'classification', model_name: 'mock-classifier-tiny',
      prompt_version: 'p1', latency_ms: 1, input_tokens: 1, output_tokens: 1,
      estimated_cost_usd: 0.50, schema_valid: false
    });
    assert.equal(await store.sumByModel('u1', 'mock-classifier-tiny'), 0.50);
  });

  it('respects capacity (oldest evicted)', async () => {
    const store = new InMemoryCostStore(3);
    for (let i = 0; i < 5; i++) {
      await store.write({
        user_id: 'u1', capability: 'classification', model_name: `mock-${i}`,
        prompt_version: 'p1', latency_ms: 1, input_tokens: 1, output_tokens: 1,
        estimated_cost_usd: 0.10, schema_valid: true
      });
    }
    const out = await store.recent('u1');
    assert.equal(out.length, 3);
    assert.deepEqual(out.map((r) => r.model_name), ['mock-4', 'mock-3', 'mock-2']);
  });

  it('returned cost records are frozen', async () => {
    const store = new InMemoryCostStore();
    await store.write({
      user_id: 'u1', capability: 'classification', model_name: 'mock-classifier-tiny',
      prompt_version: 'p1', latency_ms: 1, input_tokens: 1, output_tokens: 1,
      estimated_cost_usd: 0.10, schema_valid: true
    });
    const [r] = await store.recent('u1');
    assert.ok(r);
    assert.throws(() => {
      (r as unknown as { estimated_cost_usd: number }).estimated_cost_usd = 999;
    });
  });
});
