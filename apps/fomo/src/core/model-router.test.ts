import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryCostStore } from './cost-tracking.ts';
import {
  type BackendResult,
  type ModelBackend,
  type ModelOutputValidator,
  createModelRouter
} from './model-router.ts';

// Minimal hand-rolled validator helpers — the router is library-agnostic, so
// tests use whatever shape is convenient.
const isNonEmptyJsonObject: ModelOutputValidator<{ label: string }> = (text: string) => {
  try {
    const parsed = JSON.parse(text);
    if (typeof parsed !== 'object' || parsed === null) {
      return { ok: false, reason: 'not an object' };
    }
    if (typeof (parsed as Record<string, unknown>).label !== 'string') {
      return { ok: false, reason: 'missing string field "label"' };
    }
    return { ok: true, value: parsed as { label: string } };
  } catch (err) {
    return { ok: false, reason: `not JSON: ${err instanceof Error ? err.message : String(err)}` };
  }
};

function makeBackend(opts: {
  name?: string;
  model_name?: string;
  text?: string;
  input_tokens?: number;
  output_tokens?: number;
  latency_ms?: number;
  throws?: Error;
  delay_ms?: number;
}): ModelBackend {
  const modelName = opts.model_name ?? 'mock-classifier-tiny';
  const name = opts.name ?? modelName;
  return {
    name: () => name,
    async call(): Promise<BackendResult> {
      // This test helper ignores the request argument; the real MockModelBackend
      // class tested in model-backends/mock.test.ts exercises prompt + timeout
      // routing properly. TypeScript's contravariant function-param rules let
      // an implementation accept fewer params than its interface.
      if (opts.delay_ms && opts.delay_ms > 0) {
        await new Promise((resolve) => setTimeout(resolve, opts.delay_ms));
      }
      if (opts.throws) throw opts.throws;
      return {
        text: opts.text ?? '{"label":"important"}',
        input_tokens: opts.input_tokens ?? 100,
        output_tokens: opts.output_tokens ?? 20,
        model_name: modelName,
        latency_ms: opts.latency_ms ?? 50
      };
    }
  };
}

describe('createModelRouter — registration', () => {
  it('rejects registration with an unknown capability tag', () => {
    const router = createModelRouter({ costStore: new InMemoryCostStore() });
    assert.throws(
      () => router.registerBackend('planning' as never, makeBackend({})),
      /unknown capability tag/
    );
  });

  it('accepts registration for the v0.1 tag classification', () => {
    const router = createModelRouter({ costStore: new InMemoryCostStore() });
    router.registerBackend('classification', makeBackend({}));
  });
});

describe('createModelRouter — route success path', () => {
  it('returns ok with parsed output, model_name, latency, tokens, cost', async () => {
    const costStore = new InMemoryCostStore();
    const router = createModelRouter({ costStore });
    router.registerBackend('classification', makeBackend({
      text: '{"label":"important"}',
      input_tokens: 250,
      output_tokens: 8,
      latency_ms: 73
    }));

    const result = await router.route({
      capability: 'classification',
      prompt: 'should I care about this email?',
      prompt_version: 'p1',
      user_id: 'u1',
      validate: isNonEmptyJsonObject
    });

    assert.equal(result.ok, true);
    if (result.ok) {
      assert.deepEqual(result.output, { label: 'important' });
      assert.equal(result.model_name, 'mock-classifier-tiny');
      assert.equal(result.latency_ms, 73);
      assert.equal(result.input_tokens, 250);
      assert.equal(result.output_tokens, 8);
      assert.ok(result.estimated_cost_usd > 0);
    }
  });

  it('writes a cost record per successful call', async () => {
    const costStore = new InMemoryCostStore();
    const router = createModelRouter({ costStore });
    router.registerBackend('classification', makeBackend({ input_tokens: 500, output_tokens: 50 }));

    await router.route({
      capability: 'classification',
      prompt: 'x',
      prompt_version: 'p2',
      user_id: 'u1',
      validate: isNonEmptyJsonObject
    });

    const records = await costStore.recent('u1');
    assert.equal(records.length, 1);
    assert.equal(records[0]?.prompt_version, 'p2');
    assert.equal(records[0]?.schema_valid, true);
    assert.equal(records[0]?.input_tokens, 500);
    assert.equal(records[0]?.output_tokens, 50);
  });
});

describe('createModelRouter — fail-closed paths', () => {
  it('denies unknown_capability when caller passes a tag outside v0.1', async () => {
    const router = createModelRouter({ costStore: new InMemoryCostStore() });
    router.registerBackend('classification', makeBackend({}));
    const result = await router.route({
      capability: 'planning' as never,
      prompt: 'x',
      prompt_version: 'p1',
      user_id: 'u1',
      validate: isNonEmptyJsonObject
    });
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'unknown_capability');
      assert.equal(result.model_name, null);
    }
  });

  it('denies no_backend_for_capability when nothing is registered', async () => {
    const router = createModelRouter({ costStore: new InMemoryCostStore() });
    const result = await router.route({
      capability: 'classification',
      prompt: 'x',
      prompt_version: 'p1',
      user_id: 'u1',
      validate: isNonEmptyJsonObject
    });
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'no_backend_for_capability');
      assert.equal(result.model_name, null);
    }
  });

  it('denies backend_error when the backend throws', async () => {
    const router = createModelRouter({ costStore: new InMemoryCostStore() });
    router.registerBackend(
      'classification',
      makeBackend({ throws: new Error('upstream 503') })
    );
    const result = await router.route({
      capability: 'classification',
      prompt: 'x',
      prompt_version: 'p1',
      user_id: 'u1',
      validate: isNonEmptyJsonObject
    });
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'backend_error');
      assert.match(result.reason, /upstream 503/);
      assert.equal(result.model_name, 'mock-classifier-tiny');
    }
  });

  it('denies timeout when the backend exceeds the timeout window', async () => {
    const router = createModelRouter({ costStore: new InMemoryCostStore() });
    router.registerBackend('classification', makeBackend({ delay_ms: 200 }));
    const result = await router.route({
      capability: 'classification',
      prompt: 'x',
      prompt_version: 'p1',
      user_id: 'u1',
      validate: isNonEmptyJsonObject,
      timeout_ms: 30
    });
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'timeout');
      assert.match(result.reason, /exceeded timeout of 30ms/);
      assert.equal(result.model_name, 'mock-classifier-tiny');
    }
  });

  it('denies schema_invalid when the backend output fails the validator', async () => {
    const costStore = new InMemoryCostStore();
    const router = createModelRouter({ costStore });
    router.registerBackend('classification', makeBackend({ text: 'not json at all' }));
    const result = await router.route({
      capability: 'classification',
      prompt: 'x',
      prompt_version: 'p1',
      user_id: 'u1',
      validate: isNonEmptyJsonObject
    });
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'schema_invalid');
      assert.match(result.reason, /not JSON/);
      assert.equal(result.model_name, 'mock-classifier-tiny');
    }
  });

  it('still writes a cost record on schema_invalid (wasted spend is real spend)', async () => {
    const costStore = new InMemoryCostStore();
    const router = createModelRouter({ costStore });
    router.registerBackend('classification', makeBackend({
      text: 'garbage',
      input_tokens: 400,
      output_tokens: 10
    }));
    await router.route({
      capability: 'classification',
      prompt: 'x',
      prompt_version: 'p1',
      user_id: 'u1',
      validate: isNonEmptyJsonObject
    });
    const records = await costStore.recent('u1');
    assert.equal(records.length, 1);
    assert.equal(records[0]?.schema_valid, false);
    assert.equal(records[0]?.input_tokens, 400);
  });

  it('does NOT write a cost record on backend_error (no tokens consumed)', async () => {
    const costStore = new InMemoryCostStore();
    const router = createModelRouter({ costStore });
    router.registerBackend('classification', makeBackend({ throws: new Error('boom') }));
    await router.route({
      capability: 'classification',
      prompt: 'x',
      prompt_version: 'p1',
      user_id: 'u1',
      validate: isNonEmptyJsonObject
    });
    const records = await costStore.recent('u1');
    assert.equal(records.length, 0);
  });

  it('does NOT write a cost record on timeout (backend did not complete)', async () => {
    const costStore = new InMemoryCostStore();
    const router = createModelRouter({ costStore });
    router.registerBackend('classification', makeBackend({ delay_ms: 200 }));
    await router.route({
      capability: 'classification',
      prompt: 'x',
      prompt_version: 'p1',
      user_id: 'u1',
      validate: isNonEmptyJsonObject,
      timeout_ms: 30
    });
    const records = await costStore.recent('u1');
    assert.equal(records.length, 0);
  });

  it('does NOT write a cost record on unknown_capability / no_backend (never reached the backend)', async () => {
    const costStore = new InMemoryCostStore();
    const router = createModelRouter({ costStore });
    await router.route({
      capability: 'planning' as never,
      prompt: 'x',
      prompt_version: 'p1',
      user_id: 'u1',
      validate: isNonEmptyJsonObject
    });
    await router.route({
      capability: 'classification',
      prompt: 'x',
      prompt_version: 'p1',
      user_id: 'u1',
      validate: isNonEmptyJsonObject
    });
    assert.equal((await costStore.recent('u1')).length, 0);
  });
});

describe('createModelRouter — result objects are frozen', () => {
  it('success result cannot be mutated', async () => {
    const router = createModelRouter({ costStore: new InMemoryCostStore() });
    router.registerBackend('classification', makeBackend({}));
    const result = await router.route({
      capability: 'classification',
      prompt: 'x',
      prompt_version: 'p1',
      user_id: 'u1',
      validate: isNonEmptyJsonObject
    });
    assert.throws(() => {
      (result as unknown as { ok: boolean }).ok = false;
    });
  });

  it('error result cannot be mutated', async () => {
    const router = createModelRouter({ costStore: new InMemoryCostStore() });
    const result = await router.route({
      capability: 'classification',
      prompt: 'x',
      prompt_version: 'p1',
      user_id: 'u1',
      validate: isNonEmptyJsonObject
    });
    assert.throws(() => {
      (result as unknown as { code: string }).code = 'allowed';
    });
  });
});
