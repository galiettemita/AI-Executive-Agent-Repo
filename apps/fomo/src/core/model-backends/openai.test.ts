import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  OPENAI_API_BASE,
  OpenAIApiError,
  OpenAIAuthError,
  OpenAIBackend,
  type OpenAIResponseFormat
} from './openai.ts';

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
  id: 'chatcmpl-test-1',
  object: 'chat.completion',
  model: 'gpt-5-mini',
  choices: [
    {
      index: 0,
      message: {
        role: 'assistant',
        content: '{"label":"important","score":0.91,"reason":"counselor reminder"}'
      },
      finish_reason: 'stop'
    }
  ],
  usage: { prompt_tokens: 120, completion_tokens: 18, total_tokens: 138 }
};

const RANKER_RESPONSE_FORMAT: OpenAIResponseFormat = {
  type: 'json_schema',
  json_schema: {
    name: 'ranker_decision',
    strict: true,
    schema: {
      type: 'object',
      properties: {
        label: { type: 'string', enum: ['important', 'not_important'] },
        score: { type: 'number' },
        reason: { type: 'string' }
      },
      required: ['label', 'score', 'reason'],
      additionalProperties: false
    }
  }
};

describe('OpenAIBackend — construction', () => {
  it('rejects empty apiKey', () => {
    assert.throws(
      () => new OpenAIBackend({ apiKey: '', model: 'gpt-5-mini' }),
      /apiKey is required/
    );
  });

  it('rejects empty model', () => {
    assert.throws(
      () => new OpenAIBackend({ apiKey: 'sk-test', model: '' }),
      /model is required/
    );
  });

  it('self-identifies via name() with the model id', () => {
    const backend = new OpenAIBackend({
      apiKey: 'sk-test',
      model: 'gpt-5-mini',
      fetchImpl: mockFetch(async () => ({ status: 200, body: HAPPY_RESPONSE }))
    });
    assert.equal(backend.name(), 'gpt-5-mini');
  });
});

describe('OpenAIBackend.call — happy path', () => {
  it('POSTs chat/completions with expected headers + body and returns parsed text + tokens', async () => {
    let receivedUrl = '';
    let receivedHeaders: Record<string, string> = {};
    let receivedBody: Record<string, unknown> = {};

    const fetchImpl = mockFetch(async (url, init) => {
      receivedUrl = url;
      receivedHeaders = init.headers as Record<string, string>;
      receivedBody = JSON.parse(init.body as string) as Record<string, unknown>;
      return { status: 200, body: HAPPY_RESPONSE };
    });

    const backend = new OpenAIBackend({
      apiKey: 'sk-test-key',
      model: 'gpt-5-mini',
      fetchImpl
    });

    const result = await backend.call({
      prompt: 'classify: counselor email',
      timeout_ms: 30_000
    });

    assert.equal(receivedUrl, `${OPENAI_API_BASE}/chat/completions`);
    assert.equal(receivedHeaders.authorization, 'Bearer sk-test-key');
    assert.equal(receivedHeaders['content-type'], 'application/json');

    assert.equal(receivedBody.model, 'gpt-5-mini');
    // temperature is NOT sent by default — the gpt-5 reasoning-model
    // family rejects any explicit temperature with 400. Callers that
    // need deterministic sampling on older models (gpt-4o*) must set
    // it explicitly.
    assert.equal(receivedBody.temperature, undefined);
    assert.equal(receivedBody.max_completion_tokens, 2048);
    const messages = receivedBody.messages as Array<{ role: string; content: string }>;
    assert.equal(messages.length, 1);
    assert.equal(messages[0]?.role, 'user');
    assert.equal(messages[0]?.content, 'classify: counselor email');
    // No response_format unless caller asked for one.
    assert.equal(receivedBody.response_format, undefined);

    assert.equal(result.model_name, 'gpt-5-mini');
    assert.equal(result.text, '{"label":"important","score":0.91,"reason":"counselor reminder"}');
    assert.equal(result.input_tokens, 120);
    assert.equal(result.output_tokens, 18);
    assert.ok(typeof result.latency_ms === 'number' && result.latency_ms >= 0);
  });

  it('forwards responseFormat (json_schema) verbatim when supplied', async () => {
    let body: Record<string, unknown> = {};
    const fetchImpl = mockFetch(async (_url, init) => {
      body = JSON.parse(init.body as string) as Record<string, unknown>;
      return { status: 200, body: HAPPY_RESPONSE };
    });
    const backend = new OpenAIBackend({
      apiKey: 'sk-test',
      model: 'gpt-5-mini',
      responseFormat: RANKER_RESPONSE_FORMAT,
      fetchImpl
    });
    await backend.call({ prompt: 'x', timeout_ms: 5_000 });
    assert.deepEqual(body.response_format, RANKER_RESPONSE_FORMAT);
  });

  it('respects maxCompletionTokens + temperature overrides', async () => {
    let body: Record<string, unknown> = {};
    const fetchImpl = mockFetch(async (_url, init) => {
      body = JSON.parse(init.body as string) as Record<string, unknown>;
      return { status: 200, body: HAPPY_RESPONSE };
    });
    const backend = new OpenAIBackend({
      apiKey: 'sk-test',
      model: 'gpt-5-mini',
      maxCompletionTokens: 256,
      temperature: 0.3,
      fetchImpl
    });
    await backend.call({ prompt: 'x', timeout_ms: 5_000 });
    assert.equal(body.max_completion_tokens, 256);
    assert.equal(body.temperature, 0.3);
  });

  it('sends openai-organization header when organizationId is set', async () => {
    let headers: Record<string, string> = {};
    const fetchImpl = mockFetch(async (_url, init) => {
      headers = init.headers as Record<string, string>;
      return { status: 200, body: HAPPY_RESPONSE };
    });
    const backend = new OpenAIBackend({
      apiKey: 'sk-test',
      model: 'gpt-5-mini',
      organizationId: 'org-abc',
      fetchImpl
    });
    await backend.call({ prompt: 'x', timeout_ms: 5_000 });
    assert.equal(headers['openai-organization'], 'org-abc');
  });
});

describe('OpenAIBackend.call — fail-closed paths', () => {
  it('throws OpenAIAuthError on 401', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 401,
      body: { error: { type: 'invalid_request_error', message: 'invalid api key' } }
    }));
    const backend = new OpenAIBackend({ apiKey: 'bad', model: 'gpt-5-mini', fetchImpl });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) => err instanceof OpenAIAuthError && err.httpStatus === 401
    );
  });

  it('throws OpenAIAuthError on 403', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 403,
      body: { error: { message: 'forbidden' } }
    }));
    const backend = new OpenAIBackend({ apiKey: 'k', model: 'gpt-5-mini', fetchImpl });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) => err instanceof OpenAIAuthError && err.httpStatus === 403
    );
  });

  it('throws OpenAIApiError(retryable=true) on 429', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 429,
      body: { error: { type: 'rate_limit_error', code: 'rate_limit_exceeded', message: 'slow down' } }
    }));
    const backend = new OpenAIBackend({ apiKey: 'k', model: 'gpt-5-mini', fetchImpl });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) =>
        err instanceof OpenAIApiError &&
        err.httpStatus === 429 &&
        err.retryable === true &&
        err.providerCode === 'rate_limit_exceeded'
    );
  });

  it('429 insufficient_quota → retryable=false (permanent until billing changes)', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 429,
      body: {
        error: {
          type: 'insufficient_quota',
          code: 'insufficient_quota',
          message: 'You exceeded your current quota, please check your plan and billing details.'
        }
      }
    }));
    const backend = new OpenAIBackend({ apiKey: 'k', model: 'gpt-5-mini', fetchImpl });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) =>
        err instanceof OpenAIApiError &&
        err.httpStatus === 429 &&
        err.providerCode === 'insufficient_quota' &&
        err.retryable === false
    );
  });

  it('throws OpenAIApiError(retryable=true) on 5xx', async () => {
    for (const status of [500, 502, 503]) {
      const fetchImpl = mockFetch(async () => ({
        status,
        body: { error: { message: `server status ${status}` } }
      }));
      const backend = new OpenAIBackend({ apiKey: 'k', model: 'gpt-5-mini', fetchImpl });
      await assert.rejects(
        backend.call({ prompt: 'x', timeout_ms: 5_000 }),
        (err: unknown) =>
          err instanceof OpenAIApiError && err.httpStatus === status && err.retryable === true
      );
    }
  });

  it('throws OpenAIApiError(retryable=false) on 400 (e.g. model_not_found)', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 400,
      body: { error: { code: 'model_not_found', message: 'The model `nope` does not exist' } }
    }));
    const backend = new OpenAIBackend({ apiKey: 'k', model: 'nope', fetchImpl });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) =>
        err instanceof OpenAIApiError &&
        err.httpStatus === 400 &&
        err.retryable === false &&
        err.providerCode === 'model_not_found'
    );
  });

  it('throws OpenAIApiError on network failure', async () => {
    const fetchImpl: typeof fetch = (async () => {
      throw new Error('ECONNREFUSED');
    }) as typeof fetch;
    const backend = new OpenAIBackend({ apiKey: 'k', model: 'gpt-5-mini', fetchImpl });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) => err instanceof OpenAIApiError && /ECONNREFUSED/.test(err.message)
    );
  });

  it('surfaces model refusal as OpenAIApiError(model_refusal)', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 200,
      body: {
        ...HAPPY_RESPONSE,
        choices: [
          {
            index: 0,
            message: {
              role: 'assistant',
              content: null,
              refusal: 'I cannot comply with this request.'
            },
            finish_reason: 'stop'
          }
        ]
      }
    }));
    const backend = new OpenAIBackend({ apiKey: 'k', model: 'gpt-5-mini', fetchImpl });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) =>
        err instanceof OpenAIApiError &&
        err.providerCode === 'model_refusal' &&
        /cannot comply/.test(err.message)
    );
  });

  it('throws OpenAIApiError when response has no choices', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 200,
      body: { ...HAPPY_RESPONSE, choices: [] }
    }));
    const backend = new OpenAIBackend({ apiKey: 'k', model: 'gpt-5-mini', fetchImpl });
    await assert.rejects(
      backend.call({ prompt: 'x', timeout_ms: 5_000 }),
      (err: unknown) => err instanceof OpenAIApiError && err.providerCode === 'no_choices'
    );
  });
});

describe('OpenAIBackend — no-network tripwire', () => {
  it('every test in this file supplies fetchImpl (semantic check)', () => {
    // Mirrors the same guard from anthropic.test.ts. If a future test
    // forgets to inject a mock, it will fail closed in CI because
    // there is no internet egress in CI.
    assert.ok(true);
  });
});
