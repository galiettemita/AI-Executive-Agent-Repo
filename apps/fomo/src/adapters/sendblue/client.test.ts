import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { SendBlueClient, type FetchLike } from './client.ts';

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' }
  });
}

interface Captured {
  url: string;
  init: RequestInit | undefined;
}

function mockFetch(impl: (url: string, init?: RequestInit) => Response | Promise<Response>): {
  fetch: FetchLike;
  calls: Captured[];
} {
  const calls: Captured[] = [];
  const fetchImpl: FetchLike = async (url, init) => {
    calls.push({ url: String(url), init });
    return impl(String(url), init);
  };
  return { fetch: fetchImpl, calls };
}

describe('SendBlueClient — construction', () => {
  it('throws when apiKeyId is missing', () => {
    assert.throws(() => new SendBlueClient({ apiKeyId: '', apiSecretKey: 's' }), /apiKeyId/);
  });
  it('throws when apiSecretKey is missing', () => {
    assert.throws(() => new SendBlueClient({ apiKeyId: 'k', apiSecretKey: '' }), /apiSecretKey/);
  });
  it('throws on a non-positive timeoutMs', () => {
    assert.throws(
      () => new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', timeoutMs: 0 }),
      /timeoutMs/
    );
  });
});

describe('SendBlueClient.send — happy path', () => {
  it('returns sent when provider returns 2xx + status=QUEUED', async () => {
    const { fetch, calls } = mockFetch(() =>
      jsonResponse(200, { status: 'QUEUED', message_handle: 'handle-123' })
    );
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hello' });
    assert.equal(out.kind, 'sent');
    assert.equal(out.providerStatus, 'QUEUED');
    assert.equal(out.providerMessageHandle, 'handle-123');
    assert.equal(out.httpStatus, 200);
    // Verify the wire format.
    assert.equal(calls.length, 1);
    const init = calls[0]!.init!;
    assert.equal(init.method, 'POST');
    const headers = init.headers as Record<string, string>;
    assert.equal(headers['sb-api-key-id'], 'k');
    assert.equal(headers['sb-api-secret-key'], 's');
    assert.equal(headers['content-type'], 'application/json');
    const body = JSON.parse(init.body as string) as { number: string; content: string };
    assert.equal(body.number, '+15555550100');
    assert.equal(body.content, 'hello');
  });

  it('accepts SENT and DELIVERED as success statuses', async () => {
    for (const status of ['SENT', 'DELIVERED']) {
      const { fetch } = mockFetch(() => jsonResponse(200, { status, messageHandle: 'h' }));
      const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
      const out = await client.send({ to: '+15555550100', content: 'hi' });
      assert.equal(out.kind, 'sent', `expected sent for status=${status}`);
      assert.equal(out.providerMessageHandle, 'h');
    }
  });
});

describe('SendBlueClient.send — clear failure', () => {
  it('returns failed when provider returns 2xx + status=FAILED', async () => {
    const { fetch } = mockFetch(() => jsonResponse(200, { status: 'FAILED', error: 'invalid_number' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'failed');
    assert.equal(out.providerStatus, 'FAILED');
  });

  it('returns failed when provider returns 2xx + status=ERROR', async () => {
    const { fetch } = mockFetch(() => jsonResponse(200, { status: 'ERROR' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'failed');
    assert.equal(out.providerStatus, 'ERROR');
  });

  it('returns failed on HTTP 401 (auth — operator must rotate keys)', async () => {
    const { fetch } = mockFetch(() => jsonResponse(401, { error: 'invalid_api_key' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'failed');
    assert.equal(out.httpStatus, 401);
    assert.match(out.reason, /auth_error/);
  });

  it('returns failed on HTTP 403', async () => {
    const { fetch } = mockFetch(() => jsonResponse(403, { error: 'forbidden' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'failed');
    assert.equal(out.httpStatus, 403);
  });

  it('returns failed on HTTP 400 (client error)', async () => {
    const { fetch } = mockFetch(() => jsonResponse(400, { error: 'invalid_number' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'failed');
    assert.equal(out.httpStatus, 400);
  });
});

describe('SendBlueClient.send — send_status_unknown (load-bearing — never auto-retry)', () => {
  it('returns send_status_unknown on network failure', async () => {
    const { fetch } = mockFetch(() => {
      throw new Error('ECONNRESET');
    });
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'send_status_unknown');
    assert.equal(out.httpStatus, 0);
    assert.match(out.reason, /network_error/);
  });

  it('returns send_status_unknown on HTTP 500', async () => {
    const { fetch } = mockFetch(() => jsonResponse(500, { error: 'internal' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'send_status_unknown');
    assert.equal(out.httpStatus, 500);
  });

  it('returns send_status_unknown on HTTP 502 / 503 / 504', async () => {
    for (const status of [502, 503, 504]) {
      const { fetch } = mockFetch(() => jsonResponse(status, {}));
      const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
      const out = await client.send({ to: '+15555550100', content: 'hi' });
      assert.equal(out.kind, 'send_status_unknown', `expected unknown for HTTP ${status}`);
    }
  });

  it('returns send_status_unknown on HTTP 429 (rate-limited — never auto-retry)', async () => {
    const { fetch } = mockFetch(() => jsonResponse(429, { error: 'rate_limited' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'send_status_unknown');
    assert.equal(out.httpStatus, 429);
    assert.match(out.reason, /rate_limited/);
  });

  it('returns send_status_unknown on HTTP 2xx with unknown provider status', async () => {
    const { fetch } = mockFetch(() => jsonResponse(200, { status: 'PROCESSING' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'send_status_unknown');
    assert.equal(out.providerStatus, 'PROCESSING');
    assert.match(out.reason, /unknown_provider_status/);
  });

  it('returns send_status_unknown on HTTP 2xx with no status field', async () => {
    const { fetch } = mockFetch(() => jsonResponse(200, {}));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'send_status_unknown');
    assert.match(out.reason, /missing_provider_status_field/);
  });

  it('returns send_status_unknown on HTTP 2xx with invalid JSON body', async () => {
    const { fetch } = mockFetch(
      () =>
        new Response('not-json', {
          status: 200,
          headers: { 'content-type': 'application/json' }
        })
    );
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'send_status_unknown');
    assert.match(out.reason, /response_unparseable/);
  });

  it('returns send_status_unknown on timeout', async () => {
    const { fetch } = mockFetch(async (_url, init) => {
      // Wait until the abort signal fires.
      await new Promise<never>((_, reject) => {
        const signal = init?.signal;
        signal?.addEventListener('abort', () =>
          reject(new DOMException('aborted', 'AbortError'))
        );
      });
      throw new Error('unreachable');
    });
    const client = new SendBlueClient({
      apiKeyId: 'k',
      apiSecretKey: 's',
      fetchImpl: fetch,
      timeoutMs: 50
    });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'send_status_unknown');
    assert.match(out.reason, /timeout/);
  });
});

describe('SendBlueClient.send — argument validation (caller errors are failed, not unknown)', () => {
  it('returns failed when to is empty', async () => {
    const { fetch, calls } = mockFetch(() => jsonResponse(200, { status: 'QUEUED' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '', content: 'hi' });
    assert.equal(out.kind, 'failed');
    assert.match(out.reason, /missing destination/);
    assert.equal(calls.length, 0, 'must not call the provider');
  });

  it('returns failed when content is empty', async () => {
    const { fetch, calls } = mockFetch(() => jsonResponse(200, { status: 'QUEUED' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: '' });
    assert.equal(out.kind, 'failed');
    assert.match(out.reason, /missing content/);
    assert.equal(calls.length, 0);
  });
});

describe('SendBlueClient.send — output immutability', () => {
  it('returned outcome is frozen', async () => {
    const { fetch } = mockFetch(() => jsonResponse(200, { status: 'QUEUED' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.throws(() => {
      (out as unknown as { kind: string }).kind = 'failed';
    });
  });
});
