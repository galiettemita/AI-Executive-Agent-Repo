import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { MockModelBackend, normalizePrompt } from './mock.ts';

describe('normalizePrompt', () => {
  it('collapses whitespace and trims', () => {
    assert.equal(normalizePrompt('  hello\n\tworld   '), 'hello world');
    assert.equal(normalizePrompt('a\n\nb'), 'a b');
    assert.equal(normalizePrompt(''), '');
  });
});

describe('MockModelBackend', () => {
  it('reports its configured name', () => {
    const b = new MockModelBackend({ model_name: 'mock-classifier-tiny' });
    assert.equal(b.name(), 'mock-classifier-tiny');
  });

  it('returns the canned response for a registered prompt', async () => {
    const b = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: {
        'important?': { text: '{"label":"important"}', input_tokens: 10, output_tokens: 5 }
      }
    });
    const r = await b.call({ prompt: 'important?', timeout_ms: 1000 });
    assert.equal(r.text, '{"label":"important"}');
    assert.equal(r.input_tokens, 10);
    assert.equal(r.output_tokens, 5);
    assert.equal(r.model_name, 'mock-classifier-tiny');
    assert.equal(r.latency_ms, 0);
  });

  it('is deterministic: same prompt → same output every time', async () => {
    const b = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: {
        'x': { text: 'A', input_tokens: 1, output_tokens: 1 }
      }
    });
    const a = await b.call({ prompt: 'x', timeout_ms: 100 });
    const c = await b.call({ prompt: 'x', timeout_ms: 100 });
    assert.equal(a.text, c.text);
    assert.equal(a.input_tokens, c.input_tokens);
    assert.equal(a.output_tokens, c.output_tokens);
  });

  it('matches prompt after whitespace normalization', async () => {
    const b = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: {
        'hello world': { text: 'hi', input_tokens: 1, output_tokens: 1 }
      }
    });
    const r = await b.call({ prompt: '  hello\n\tworld  ', timeout_ms: 100 });
    assert.equal(r.text, 'hi');
  });

  it('falls back to default when no key matches', async () => {
    const b = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: { known: { text: 'known-resp', input_tokens: 1, output_tokens: 1 } },
      default: { text: 'default-resp', input_tokens: 2, output_tokens: 3 }
    });
    const r = await b.call({ prompt: 'something-else', timeout_ms: 100 });
    assert.equal(r.text, 'default-resp');
    assert.equal(r.input_tokens, 2);
    assert.equal(r.output_tokens, 3);
  });

  it('throws a clear error when no response and no default configured', async () => {
    const b = new MockModelBackend({ model_name: 'mock-classifier-tiny' });
    await assert.rejects(
      b.call({ prompt: 'x', timeout_ms: 100 }),
      /no canned response.*no default configured/
    );
  });

  it('awaits configured latency before resolving', async () => {
    const b = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: { x: { text: 'a', input_tokens: 1, output_tokens: 1, latency_ms: 25 } }
    });
    const t0 = Date.now();
    await b.call({ prompt: 'x', timeout_ms: 1000 });
    const elapsed = Date.now() - t0;
    assert.ok(elapsed >= 20, `expected ~25ms elapsed, got ${elapsed}ms`);
  });

  it('reports the configured latency_ms back to the caller', async () => {
    const b = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: { x: { text: 'a', input_tokens: 1, output_tokens: 1, latency_ms: 42 } }
    });
    const r = await b.call({ prompt: 'x', timeout_ms: 1000 });
    assert.equal(r.latency_ms, 42);
  });

  it('throws when configured latency exceeds the requested timeout (simulates timeout)', async () => {
    const b = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: { x: { text: 'a', input_tokens: 1, output_tokens: 1, latency_ms: 100 } }
    });
    await assert.rejects(
      b.call({ prompt: 'x', timeout_ms: 10 }),
      /simulated latency 100ms exceeds timeout 10ms/
    );
  });

  it('makes no network call (offline guarantee — tested via no global fetch usage)', async () => {
    // Replace global fetch with a tripwire; the mock must not invoke it.
    const original = globalThis.fetch;
    let fetchCalls = 0;
    globalThis.fetch = (async () => {
      fetchCalls++;
      throw new Error('mock backend made a network call');
    }) as typeof fetch;
    try {
      const b = new MockModelBackend({
        model_name: 'mock-classifier-tiny',
        responses: { x: { text: 'a', input_tokens: 1, output_tokens: 1 } }
      });
      await b.call({ prompt: 'x', timeout_ms: 100 });
      assert.equal(fetchCalls, 0);
    } finally {
      globalThis.fetch = original;
    }
  });
});
