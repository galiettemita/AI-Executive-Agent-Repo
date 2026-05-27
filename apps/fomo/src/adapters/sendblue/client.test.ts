import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { createHmac } from 'node:crypto';

import { SendBlueClient, type FetchLike, verifySendBlueSignature } from './client.ts';

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
    assert.throws(
      () => new SendBlueClient({ apiKeyId: '', apiSecretKey: 's', fromNumber: '+15555550001' }),
      /apiKeyId/
    );
  });
  it('throws when apiSecretKey is missing', () => {
    assert.throws(
      () => new SendBlueClient({ apiKeyId: 'k', apiSecretKey: '', fromNumber: '+15555550001' }),
      /apiSecretKey/
    );
  });
  it('throws when fromNumber is missing (3E.2 smoke-surfaced bug: SendBlue requires from_number)', () => {
    assert.throws(
      () => new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '' }),
      /fromNumber/
    );
  });
  it('throws when fromNumber is not E.164', () => {
    assert.throws(
      () => new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '5551234' }),
      /E\.164/
    );
    assert.throws(
      () => new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+abc1234567' }),
      /E\.164/
    );
  });
  it('throws on a non-positive timeoutMs', () => {
    assert.throws(
      () =>
        new SendBlueClient({
          apiKeyId: 'k',
          apiSecretKey: 's',
          fromNumber: '+15555550001',
          timeoutMs: 0
        }),
      /timeoutMs/
    );
  });
});

describe('SendBlueClient.send — happy path', () => {
  it('returns sent when provider returns 2xx + status=QUEUED', async () => {
    const { fetch, calls } = mockFetch(() =>
      jsonResponse(200, { status: 'QUEUED', message_handle: 'handle-123' })
    );
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
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
      const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
      const out = await client.send({ to: '+15555550100', content: 'hi' });
      assert.equal(out.kind, 'sent', `expected sent for status=${status}`);
      assert.equal(out.providerMessageHandle, 'h');
    }
  });
});

describe('SendBlueClient.send — clear failure', () => {
  it('returns failed when provider returns 2xx + status=FAILED', async () => {
    const { fetch } = mockFetch(() => jsonResponse(200, { status: 'FAILED', error: 'invalid_number' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'failed');
    assert.equal(out.providerStatus, 'FAILED');
  });

  it('returns failed when provider returns 2xx + status=ERROR', async () => {
    const { fetch } = mockFetch(() => jsonResponse(200, { status: 'ERROR' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'failed');
    assert.equal(out.providerStatus, 'ERROR');
  });

  it('returns failed on HTTP 401 (auth — operator must rotate keys)', async () => {
    const { fetch } = mockFetch(() => jsonResponse(401, { error: 'invalid_api_key' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'failed');
    assert.equal(out.httpStatus, 401);
    assert.match(out.reason, /auth_error/);
  });

  it('returns failed on HTTP 403', async () => {
    const { fetch } = mockFetch(() => jsonResponse(403, { error: 'forbidden' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'failed');
    assert.equal(out.httpStatus, 403);
  });

  it('returns failed on HTTP 400 (client error)', async () => {
    const { fetch } = mockFetch(() => jsonResponse(400, { error: 'invalid_number' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
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
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'send_status_unknown');
    assert.equal(out.httpStatus, 0);
    assert.match(out.reason, /network_error/);
  });

  it('returns send_status_unknown on HTTP 500', async () => {
    const { fetch } = mockFetch(() => jsonResponse(500, { error: 'internal' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'send_status_unknown');
    assert.equal(out.httpStatus, 500);
  });

  it('returns send_status_unknown on HTTP 502 / 503 / 504', async () => {
    for (const status of [502, 503, 504]) {
      const { fetch } = mockFetch(() => jsonResponse(status, {}));
      const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
      const out = await client.send({ to: '+15555550100', content: 'hi' });
      assert.equal(out.kind, 'send_status_unknown', `expected unknown for HTTP ${status}`);
    }
  });

  it('returns send_status_unknown on HTTP 429 (rate-limited — never auto-retry)', async () => {
    const { fetch } = mockFetch(() => jsonResponse(429, { error: 'rate_limited' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'send_status_unknown');
    assert.equal(out.httpStatus, 429);
    assert.match(out.reason, /rate_limited/);
  });

  it('returns send_status_unknown on HTTP 2xx with unknown provider status', async () => {
    const { fetch } = mockFetch(() => jsonResponse(200, { status: 'PROCESSING' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'send_status_unknown');
    assert.equal(out.providerStatus, 'PROCESSING');
    assert.match(out.reason, /unknown_provider_status/);
  });

  it('returns send_status_unknown on HTTP 2xx with no status field', async () => {
    const { fetch } = mockFetch(() => jsonResponse(200, {}));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
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
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
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
      fromNumber: '+15555550001',
      fetchImpl: fetch,
      timeoutMs: 50
    });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.equal(out.kind, 'send_status_unknown');
    assert.match(out.reason, /timeout/);
  });
});

describe('SendBlueClient.send — wire format (3E.2 smoke-surfaced findings)', () => {
  it('includes from_number in the POST body (REQUIRED by SendBlue)', async () => {
    const { fetch, calls } = mockFetch(() =>
      jsonResponse(200, { status: 'QUEUED', message_handle: 'h-1' })
    );
    const client = new SendBlueClient({
      apiKeyId: 'k',
      apiSecretKey: 's',
      fromNumber: '+12143547196',
      fetchImpl: fetch
    });
    await client.send({ to: '+15555550100', content: 'hello' });
    assert.equal(calls.length, 1);
    const body = JSON.parse(calls[0]!.init!.body as string) as {
      number: string;
      content: string;
      from_number: string;
    };
    assert.equal(body.number, '+15555550100');
    assert.equal(body.content, 'hello');
    assert.equal(body.from_number, '+12143547196');
  });

  it('default timeout is 30s (bumped from 10s after SendBlue free-tier observed at ~13s)', async () => {
    // We can't trigger a real 30s timeout in test. Just confirm the
    // constructor accepts the default without throwing, and that an
    // explicit 30_000 also works.
    const { fetch } = mockFetch(() => jsonResponse(200, { status: 'QUEUED' }));
    const c1 = new SendBlueClient({
      apiKeyId: 'k',
      apiSecretKey: 's',
      fromNumber: '+15555550001',
      fetchImpl: fetch
    });
    const c2 = new SendBlueClient({
      apiKeyId: 'k',
      apiSecretKey: 's',
      fromNumber: '+15555550001',
      fetchImpl: fetch,
      timeoutMs: 30_000
    });
    // Both should send happily.
    const o1 = await c1.send({ to: '+15555550100', content: 'hi' });
    const o2 = await c2.send({ to: '+15555550100', content: 'hi' });
    assert.equal(o1.kind, 'sent');
    assert.equal(o2.kind, 'sent');
  });
});

describe('SendBlueClient.send — argument validation (caller errors are failed, not unknown)', () => {
  it('returns failed when to is empty', async () => {
    const { fetch, calls } = mockFetch(() => jsonResponse(200, { status: 'QUEUED' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
    const out = await client.send({ to: '', content: 'hi' });
    assert.equal(out.kind, 'failed');
    assert.match(out.reason, /missing destination/);
    assert.equal(calls.length, 0, 'must not call the provider');
  });

  it('returns failed when content is empty', async () => {
    const { fetch, calls } = mockFetch(() => jsonResponse(200, { status: 'QUEUED' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: '' });
    assert.equal(out.kind, 'failed');
    assert.match(out.reason, /missing content/);
    assert.equal(calls.length, 0);
  });
});

describe('SendBlueClient.send — output immutability', () => {
  it('returned outcome is frozen', async () => {
    const { fetch } = mockFetch(() => jsonResponse(200, { status: 'QUEUED' }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fromNumber: '+15555550001', fetchImpl: fetch });
    const out = await client.send({ to: '+15555550100', content: 'hi' });
    assert.throws(() => {
      (out as unknown as { kind: string }).kind = 'failed';
    });
  });
});

/* ====================================================================== */
/* verifySendBlueSignature (Phase 3F.1)                                   */
/* ====================================================================== */

const TEST_SECRET = 'test-sendblue-webhook-signing-secret';

function sign(body: string, secret: string = TEST_SECRET): string {
  return createHmac('sha256', secret).update(body).digest('hex');
}

function signTs(timestamp: string, body: string, secret: string = TEST_SECRET): string {
  return createHmac('sha256', secret).update(`${timestamp}.${body}`).digest('hex');
}

describe('verifySendBlueSignature — happy path (body-only HMAC)', () => {
  it('accepts a correctly signed body with bare-hex signature', () => {
    const body = '{"message_id":"sb-1","content":"hi"}';
    const sig = sign(body);
    const r = verifySendBlueSignature({ signingSecret: TEST_SECRET, signature: sig, body });
    assert.equal(r.ok, true);
  });

  it('accepts sha256=<hex> prefix', () => {
    const body = '{"message_id":"sb-1"}';
    const sig = `sha256=${sign(body)}`;
    const r = verifySendBlueSignature({ signingSecret: TEST_SECRET, signature: sig, body });
    assert.equal(r.ok, true);
  });

  it('accepts v1=<hex> prefix', () => {
    const body = '{"message_id":"sb-1"}';
    const sig = `v1=${sign(body)}`;
    const r = verifySendBlueSignature({ signingSecret: TEST_SECRET, signature: sig, body });
    assert.equal(r.ok, true);
  });

  it('prefix is case-insensitive (SHA256= and V1=)', () => {
    const body = '{"x":1}';
    for (const prefix of ['SHA256=', 'V1=']) {
      const r = verifySendBlueSignature({
        signingSecret: TEST_SECRET,
        signature: `${prefix}${sign(body)}`,
        body
      });
      assert.equal(r.ok, true, `expected ok for prefix ${prefix}`);
    }
  });
});

describe('verifySendBlueSignature — happy path (timestamp + body HMAC)', () => {
  it('accepts a correctly signed timestamped body when timestamp is present', () => {
    const body = '{"message_id":"sb-1"}';
    const ts = '1779836881';
    const sig = signTs(ts, body);
    const r = verifySendBlueSignature({
      signingSecret: TEST_SECRET,
      signature: sig,
      body,
      timestamp: ts
    });
    assert.equal(r.ok, true);
  });
});

describe('verifySendBlueSignature — signature failures', () => {
  it('rejects empty signature with missing_signature', () => {
    const r = verifySendBlueSignature({
      signingSecret: TEST_SECRET,
      signature: '',
      body: '{}'
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'missing_signature');
  });

  it('rejects malformed hex (too short)', () => {
    const r = verifySendBlueSignature({
      signingSecret: TEST_SECRET,
      signature: 'abc123',
      body: '{}'
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'malformed_signature');
  });

  it('rejects malformed hex (non-hex chars)', () => {
    const r = verifySendBlueSignature({
      signingSecret: TEST_SECRET,
      signature: 'g'.repeat(64),
      body: '{}'
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'malformed_signature');
  });

  it('rejects wrong secret with signature_mismatch (timing-safe compare)', () => {
    const body = '{"x":1}';
    const sig = sign(body, 'wrong-secret');
    const r = verifySendBlueSignature({
      signingSecret: TEST_SECRET,
      signature: sig,
      body
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'signature_mismatch');
  });

  it('rejects tampered body (signature was computed over a different body)', () => {
    const originalBody = '{"x":1}';
    const sig = sign(originalBody);
    const tamperedBody = '{"x":99}';
    const r = verifySendBlueSignature({
      signingSecret: TEST_SECRET,
      signature: sig,
      body: tamperedBody
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'signature_mismatch');
  });
});

describe('verifySendBlueSignature — timestamp freshness (optional)', () => {
  it('accepts when timestamp is fresh and maxAgeSeconds is set', () => {
    const now = 1779836881;
    const body = '{"x":1}';
    const ts = String(now - 60); // 60s old
    const sig = signTs(String(now - 60), body);
    const r = verifySendBlueSignature({
      signingSecret: TEST_SECRET,
      signature: sig,
      body,
      timestamp: ts,
      now: () => now,
      maxAgeSeconds: 300
    });
    assert.equal(r.ok, true);
  });

  it('rejects stale timestamp', () => {
    const now = 1779836881;
    const body = '{"x":1}';
    const ts = String(now - 1000); // 1000s old, > 300s window
    const sig = signTs(ts, body);
    const r = verifySendBlueSignature({
      signingSecret: TEST_SECRET,
      signature: sig,
      body,
      timestamp: ts,
      now: () => now,
      maxAgeSeconds: 300
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'stale_timestamp');
  });

  it('rejects timestamp from the far future too', () => {
    const now = 1779836881;
    const body = '{"x":1}';
    const ts = String(now + 1000);
    const sig = signTs(ts, body);
    const r = verifySendBlueSignature({
      signingSecret: TEST_SECRET,
      signature: sig,
      body,
      timestamp: ts,
      now: () => now,
      maxAgeSeconds: 300
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'stale_timestamp');
  });

  it('rejects malformed timestamp', () => {
    const r = verifySendBlueSignature({
      signingSecret: TEST_SECRET,
      signature: sign('{}'),
      body: '{}',
      timestamp: 'not-a-timestamp',
      maxAgeSeconds: 300
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'malformed_timestamp');
  });

  it('does NOT enforce freshness when maxAgeSeconds is undefined (some webhooks lack timestamps)', () => {
    const body = '{"x":1}';
    const sig = sign(body);
    const r = verifySendBlueSignature({
      signingSecret: TEST_SECRET,
      signature: sig,
      body,
      // timestamp undefined, maxAgeSeconds undefined
    });
    assert.equal(r.ok, true);
  });
});

describe('verifySendBlueSignature — security properties', () => {
  it('timing-safe compare: different-but-same-length wrong signatures all fail uniformly', () => {
    // Both should produce signature_mismatch (not a partial-match leak).
    const body = '{"x":1}';
    const wrong1 = '0'.repeat(64);
    const wrong2 = 'f'.repeat(64);
    const r1 = verifySendBlueSignature({ signingSecret: TEST_SECRET, signature: wrong1, body });
    const r2 = verifySendBlueSignature({ signingSecret: TEST_SECRET, signature: wrong2, body });
    assert.equal(r1.ok, false);
    assert.equal(r2.ok, false);
    if (!r1.ok && !r2.ok) {
      assert.equal(r1.reason, 'signature_mismatch');
      assert.equal(r2.reason, 'signature_mismatch');
    }
  });

  it('result objects are frozen', () => {
    const ok = verifySendBlueSignature({
      signingSecret: TEST_SECRET,
      signature: sign('{}'),
      body: '{}'
    });
    assert.throws(() => {
      (ok as unknown as { ok: boolean }).ok = false;
    });
  });
});
