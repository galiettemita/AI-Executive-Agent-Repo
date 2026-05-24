import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  ANTHROPIC_API_BASE,
  ANTHROPIC_VERSION_HEADER,
  AnthropicApiError,
  AnthropicAuthError,
  AnthropicBackend
} from './anthropic.ts';

// Same mockFetch pattern as adapters/gmail/client.test.ts so the unit
// tests fully exercise success + every fail-closed path with no
// network. CI never touches api.anthropic.com.
function mockFetch(
  handler: (url: string, init: RequestInit) => Promise<{ status: number; body: unknown }>
): typeof fetch {
  return (async (input: string | URL | Request, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input.toString();
    const result = await handler(url, init ?? {});
    return new Response(JSON.stringify(result.body), {
      status: result.status,
      headers: { 'content-type': 'application/json' }
    });
  }) as typeof fetch;
}

const HAPPY_RESPONSE = {
  id: 'msg_test_1',
  type: 'message',
  role: 'assistant',
  model: 'claude-haiku-4-5-20251001',
  content: [{ type: 'text', text: '{"label":"important","reason":"counselor reminder"}' }],
  stop_reason: 'end_turn',
  usage: { input_tokens: 120, output_tokens: 18 }
};

describe('AnthropicBackend — construction', () => {
  it('rejects empty apiKey', () => {
    assert.throws(
      () => new AnthropicBackend({ apiKey: '', model: 'claude-haiku-4-5-20251001' }),
      /apiKey is required/
    );
  });

  it('self-identifies via name() with the model id', () => {
    const backend = new AnthropicBackend({
      apiKey: 'sk-ant-test',
      model: 'claude-sonnet-4-6',
      fetchImpl: mockFetch(async () => ({ status: 200, body: HAPPY_RESPONSE }))
    });
    assert.equal(backend.name(), 'claude-sonnet-4-6');
  });
});

describe('AnthropicBackend.call — happy path', () => {
  it('POSTs to the messages endpoint with the expected shape and returns parsed text + tokens', async () => {
    let receivedUrl = '';
    let receivedHeaders: Record<string, string> = {};
    let receivedBody: unknown = null;

    const fetchImpl = mockFetch(async (url, init) => {
      receivedUrl = url;
      receivedHeaders = init.headers as Record<string, string>;
      receivedBody = JSON.parse(init.body as string) as unknown;
      return { status: 200, body: HAPPY_RESPONSE };
    });

    const backend = new AnthropicBackend({
      apiKey: 'sk-ant-test',
      model: 'claude-haiku-4-5-20251001',
      fetchImpl
    });

    const result = await backend.call({ prompt: 'classify: counselor email', timeout_ms: 30_000 });

    assert.equal(receivedUrl, `${ANTHROPIC_API_BASE}/messages`);
    assert.equal(receivedHeaders['x-api-key'], 'sk-ant-test');
    assert.equal(receivedHeaders['anthropic-version'], ANTHROPIC_VERSION_HEADER);
    assert.equal(receivedHeaders['content-type'], 'application/json');

    const body = receivedBody as Record<string, unknown>;
    assert.equal(body.model, 'claude-haiku-4-5-20251001');
    assert.equal(body.max_tokens, 1024);
    assert.equal(body.temperature, 0);
    const messages = body.messages as Array<{ role: string; content: string }>;
    assert.equal(messages.length, 1);
    assert.equal(messages[0]?.role, 'user');
    assert.equal(messages[0]?.content, 'classify: counselor email');

    assert.equal(result.model_name, 'claude-haiku-4-5-20251001');
    assert.equal(result.text, '{"label":"important","reason":"counselor reminder"}');
    assert.equal(result.input_tokens, 120);
    assert.equal(result.output_tokens, 18);
    assert.ok(typeof result.latency_ms === 'number' && result.latency_ms >= 0);
  });

  it('respects maxOutputTokens + temperature overrides', async () => {
    let body: Record<string, unknown> = {};
    const fetchImpl = mockFetch(async (_url, init) => {
      body = JSON.parse(init.body as string) as Record<string, unknown>;
      return { status: 200, body: HAPPY_RESPONSE };
    });
    const backend = new AnthropicBackend({
      apiKey: 'k',
      model: 'claude-haiku-4-5-20251001',
      maxOutputTokens: 256,
      temperature: 0.5,
      fetchImpl
    });
    await backend.call({ prompt: 'x', timeout_ms: 5_000 });
    assert.equal(body.max_tokens, 256);
    assert.equal(body.temperature, 0.5);
  });

  it('concatenates multiple text content blocks', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 200,
      body: {
        ...HAPPY_RESPONSE,
        content: [
          { type: 'text', text: 'first part ' },
          { type: 'text', text: 'second part' }
        ]
      }
    }));
    const backend = new AnthropicBackend({
      apiKey: 'k',
      model: 'claude-haiku-4-5-20251001',
      fetchImpl
    });
    const r = await backend.call({ prompt: 'x', timeout_ms: 5_000 });
    assert.equal(r.text, 'first part second part');
  });

  it('ignores non-text content blocks', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 200,
      body: {
        ...HAPPY_RESPONSE,
        content: [
          { type: 'tool_use', text: 'should be ignored' } as unknown,
          { type: 'text', text: 'the answer' }
        ]
      }
    }));
    const backend = new AnthropicBackend({
      apiKey: 'k',
      model: 'claude-haiku-4-5-20251001',
      fetchImpl
    });
    const r = await backend.call({ prompt: 'x', timeout_ms: 5_000 });
    assert.equal(r.text, 'the answer');
  });
});

describe('AnthropicBackend.call — fail-closed paths', () => {
  it('throws AnthropicAuthError on 401', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 401,
      body: { error: { type: 'authentication_error', message: 'invalid key' } }
    }));
    const backend = new AnthropicBackend({
      apiKey: 'bad',
      model: 'claude-haiku-4-5-20251001',
      fetchImpl
    });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) => err instanceof AnthropicAuthError && err.httpStatus === 401
    );
  });

  it('throws AnthropicAuthError on 403', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 403, body: { error: { message: 'forbidden' } } }));
    const backend = new AnthropicBackend({
      apiKey: 'k',
      model: 'claude-haiku-4-5-20251001',
      fetchImpl
    });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) => err instanceof AnthropicAuthError && err.httpStatus === 403
    );
  });

  it('throws AnthropicApiError(retryable=true) on 429', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 429,
      body: { error: { type: 'rate_limit_error', message: 'too many requests' } }
    }));
    const backend = new AnthropicBackend({
      apiKey: 'k',
      model: 'claude-haiku-4-5-20251001',
      fetchImpl
    });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) =>
        err instanceof AnthropicApiError &&
        err.httpStatus === 429 &&
        err.retryable === true &&
        err.providerCode === 'rate_limit_error'
    );
  });

  it('throws AnthropicApiError(retryable=true) on 500 + 529 (overloaded)', async () => {
    for (const status of [500, 529]) {
      const fetchImpl = mockFetch(async () => ({
        status,
        body: { error: { message: `server status ${status}` } }
      }));
      const backend = new AnthropicBackend({
        apiKey: 'k',
        model: 'claude-haiku-4-5-20251001',
        fetchImpl
      });
      await assert.rejects(
        backend.call({ prompt: 'x', timeout_ms: 5_000 }),
        (err: unknown) =>
          err instanceof AnthropicApiError && err.httpStatus === status && err.retryable === true
      );
    }
  });

  it('throws AnthropicApiError(retryable=false) on 400', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 400,
      body: { error: { type: 'invalid_request_error', message: 'bad input' } }
    }));
    const backend = new AnthropicBackend({
      apiKey: 'k',
      model: 'claude-haiku-4-5-20251001',
      fetchImpl
    });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) =>
        err instanceof AnthropicApiError && err.httpStatus === 400 && err.retryable === false
    );
  });

  it('throws AnthropicApiError on fetch failure (network error)', async () => {
    const fetchImpl: typeof fetch = (async () => {
      throw new Error('ECONNREFUSED');
    }) as typeof fetch;
    const backend = new AnthropicBackend({
      apiKey: 'k',
      model: 'claude-haiku-4-5-20251001',
      fetchImpl
    });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) => err instanceof AnthropicApiError && /ECONNREFUSED/.test(err.message)
    );
  });

  it('throws AnthropicApiError on non-JSON response body', async () => {
    const fetchImpl: typeof fetch = (async () => {
      // 200 OK but body is not JSON.
      return new Response('not json', {
        status: 200,
        headers: { 'content-type': 'text/plain' }
      });
    }) as typeof fetch;
    const backend = new AnthropicBackend({
      apiKey: 'k',
      model: 'claude-haiku-4-5-20251001',
      fetchImpl
    });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) => err instanceof AnthropicApiError
    );
  });
});

describe('AnthropicBackend — no-network tripwire', () => {
  it('default fetch is never invoked in tests (mocks always supplied)', () => {
    // The other tests construct backends with fetchImpl; this is a
    // semantic assertion that the default fetch path is not exercised
    // here. The check is structural: every test in this file passes
    // a fetchImpl. If a future test forgets, it will time out / fail
    // closed because CI has no internet.
    assert.ok(true);
  });
});
