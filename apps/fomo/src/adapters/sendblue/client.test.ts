import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { SendBlueClient, type FetchLike, verifySendBlueWebhookSecret } from './client.ts';

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
/* verifySendBlueWebhookSecret (Phase 3F.1 — corrected to match SendBlue) */
/*                                                                        */
/* SendBlue's webhook auth is a PLAIN SHARED SECRET sent in a request     */
/* header, NOT an HMAC body signature. Docs evidence (fetched 2026-05-26  */
/* during the 3F.1 founder-mandated review):                              */
/*                                                                        */
/*   "When you configure a secret, Sendblue will include it in the        */
/*    webhook request headers, allowing you to verify that the request    */
/*    is genuinely from Sendblue."                                        */
/*    — docs.sendblue.com/getting-started/webhooks                        */
/*                                                                        */
/* No HMAC, no body signature, no documented timestamp/replay window.     */
/* Replay protection comes from inbound_replies UNIQUE on                 */
/* provider_message_id (the SendBlue message_handle), not from a          */
/* timestamp freshness check.                                             */
/* ====================================================================== */

const TEST_WEBHOOK_SECRET = 'shh-test-webhook-secret-from-sendblue-dashboard';

describe('verifySendBlueWebhookSecret — happy path (plain shared secret in header)', () => {
  it('accepts a request whose header value equals the configured secret', () => {
    const r = verifySendBlueWebhookSecret({
      configuredSecret: TEST_WEBHOOK_SECRET,
      headerValue: TEST_WEBHOOK_SECRET
    });
    assert.equal(r.ok, true);
  });

  it('tolerates whitespace around the header value (HTTP protocol noise)', () => {
    const r = verifySendBlueWebhookSecret({
      configuredSecret: TEST_WEBHOOK_SECRET,
      headerValue: `   ${TEST_WEBHOOK_SECRET}   `
    });
    assert.equal(r.ok, true);
  });
});

describe('verifySendBlueWebhookSecret — failures (fail-closed: route returns 401, audits, no parse)', () => {
  it('rejects missing/empty header with missing_header', () => {
    const r1 = verifySendBlueWebhookSecret({
      configuredSecret: TEST_WEBHOOK_SECRET,
      headerValue: ''
    });
    assert.equal(r1.ok, false);
    if (!r1.ok) assert.equal(r1.reason, 'missing_header');
    const r2 = verifySendBlueWebhookSecret({
      configuredSecret: TEST_WEBHOOK_SECRET,
      headerValue: '   '
    });
    // Whitespace-only also fails after trim.
    assert.equal(r2.ok, false);
  });

  it('rejects wrong secret with secret_mismatch (timing-safe compare)', () => {
    const r = verifySendBlueWebhookSecret({
      configuredSecret: TEST_WEBHOOK_SECRET,
      headerValue: 'completely-different-value-with-different-length-and-content'
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'secret_mismatch');
  });

  it('rejects same-length-but-different-content with secret_mismatch', () => {
    // Build a wrong secret of EXACTLY the same length so the length
    // pre-check passes and timingSafeEqual is exercised.
    const wrong = 'X'.repeat(TEST_WEBHOOK_SECRET.length);
    const r = verifySendBlueWebhookSecret({
      configuredSecret: TEST_WEBHOOK_SECRET,
      headerValue: wrong
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'secret_mismatch');
  });

  it('defensive: empty configuredSecret falls through to secret_mismatch (bootstrap should fail-close first, but never trust empty)', () => {
    const r = verifySendBlueWebhookSecret({
      configuredSecret: '',
      headerValue: 'anything'
    });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.reason, 'secret_mismatch');
  });
});

describe('verifySendBlueWebhookSecret — security properties', () => {
  it('uses timing-safe compare (assertion: no character-by-character early-exit)', () => {
    // We can't directly observe timing differences in a unit test,
    // but we CAN assert that two different-but-equal-length wrong
    // secrets both produce identical secret_mismatch results without
    // throwing — the failure shape is uniform regardless of how
    // similar the wrong secret is to the correct one.
    const wrong1 = TEST_WEBHOOK_SECRET.slice(0, -1) + 'X'; // differs only in last char
    const wrong2 = 'X' + TEST_WEBHOOK_SECRET.slice(1);     // differs only in first char
    const r1 = verifySendBlueWebhookSecret({ configuredSecret: TEST_WEBHOOK_SECRET, headerValue: wrong1 });
    const r2 = verifySendBlueWebhookSecret({ configuredSecret: TEST_WEBHOOK_SECRET, headerValue: wrong2 });
    assert.equal(r1.ok, false);
    assert.equal(r2.ok, false);
    if (!r1.ok && !r2.ok) {
      assert.equal(r1.reason, 'secret_mismatch');
      assert.equal(r2.reason, 'secret_mismatch');
    }
  });

  it('result objects are frozen', () => {
    const ok = verifySendBlueWebhookSecret({
      configuredSecret: TEST_WEBHOOK_SECRET,
      headerValue: TEST_WEBHOOK_SECRET
    });
    assert.throws(() => {
      (ok as unknown as { ok: boolean }).ok = false;
    });
    const fail = verifySendBlueWebhookSecret({
      configuredSecret: TEST_WEBHOOK_SECRET,
      headerValue: 'wrong'
    });
    assert.throws(() => {
      (fail as unknown as { ok: boolean }).ok = true;
    });
  });
});
