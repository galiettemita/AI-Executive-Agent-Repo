import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { GmailClient, GMAIL_READONLY_SCOPE } from '../adapters/gmail/client.ts';
import { SendBlueClient } from '../adapters/sendblue/client.ts';
import { loadCryptoConfig } from '../security/token-crypto.ts';
import { InMemoryTokenStore } from '../security/oauth/token-store.ts';
import {
  type DispatchContext
} from './dispatcher.ts';
import {
  GmailReadTokenMissingError,
  gmailReadExecutor,
  sendBlueSendExecutor
} from './external-executors.ts';

const TEST_KEK = Buffer.alloc(32, 7).toString('base64');

function withEnv<T>(env: Record<string, string | undefined>, fn: () => T): T {
  const previous: Record<string, string | undefined> = {};
  for (const [k, v] of Object.entries(env)) {
    previous[k] = process.env[k];
    if (v === undefined) delete process.env[k];
    else process.env[k] = v;
  }
  try {
    return fn();
  } finally {
    for (const [k, v] of Object.entries(previous)) {
      if (v === undefined) delete process.env[k];
      else process.env[k] = v;
    }
  }
}

const cryptoConfig = withEnv(
  { BREVIO_TOKEN_KEK: TEST_KEK, BREVIO_DEV_MODE: undefined },
  () => loadCryptoConfig()
);

const CTX: DispatchContext = Object.freeze({
  user_id: 'u-1',
  invocation_id: 'inv-1'
});

// Synthesizes a fetch that asserts call shape and returns a fixed body.
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

async function seedToken(store: InMemoryTokenStore, user_id: string, token: string): Promise<void> {
  await store.save({
    user_id,
    provider: 'google',
    scopes: [GMAIL_READONLY_SCOPE],
    access_token: token
  });
}

// A minimal valid Gmail messages.get JSON, base64url-decodes to "Hi Sarah".
const FAKE_MESSAGE = {
  id: 'msg-abc',
  threadId: 'thr-1',
  internalDate: '1700000000000',
  payload: {
    headers: [
      { name: 'From', value: 'Sarah <sarah@example.com>' },
      { name: 'Subject', value: 'lunch?' }
    ],
    mimeType: 'text/plain',
    body: { data: 'SGkgU2FyYWg' } // base64url('Hi Sarah')
  }
};

describe('gmailReadExecutor — happy path', () => {
  it('loads access token, calls GmailClient.getMessage, returns RawEmailContext', async () => {
    let receivedUrl = '';
    let receivedAuth = '';
    const fetchImpl = mockFetch(async (url, init) => {
      receivedUrl = url;
      receivedAuth = (init.headers as Record<string, string>).authorization;
      return { status: 200, body: FAKE_MESSAGE };
    });
    const client = new GmailClient({ fetchImpl });
    const tokenStore = new InMemoryTokenStore(cryptoConfig);
    await seedToken(tokenStore, 'u-1', 'real-access-token');

    const exec = gmailReadExecutor({ client, tokenStore });
    const result = await exec({ message_id: 'msg-abc' }, CTX);

    assert.match(receivedUrl, /\/users\/me\/messages\/msg-abc\?format=full$/);
    assert.equal(receivedAuth, 'Bearer real-access-token');
    assert.equal(result.message_id, 'msg-abc');
    assert.equal(result.sender_email, 'sarah@example.com');
    assert.equal(result.sender_name, 'Sarah');
    assert.equal(result.subject, 'lunch?');
    assert.equal(result.body_plain, 'Hi Sarah');
  });
});

describe('gmailReadExecutor — input validation', () => {
  it('throws when args.message_id is missing', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 200, body: FAKE_MESSAGE }));
    const client = new GmailClient({ fetchImpl });
    const tokenStore = new InMemoryTokenStore(cryptoConfig);
    await seedToken(tokenStore, 'u-1', 't');

    const exec = gmailReadExecutor({ client, tokenStore });
    await assert.rejects(
      () => exec({} as { message_id: string }, CTX),
      /args\.message_id is required/
    );
  });

  it('throws when args.message_id is the empty string', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 200, body: FAKE_MESSAGE }));
    const client = new GmailClient({ fetchImpl });
    const tokenStore = new InMemoryTokenStore(cryptoConfig);
    await seedToken(tokenStore, 'u-1', 't');

    const exec = gmailReadExecutor({ client, tokenStore });
    await assert.rejects(
      () => exec({ message_id: '' }, CTX),
      /args\.message_id is required/
    );
  });
});

describe('gmailReadExecutor — fail-closed paths', () => {
  it('throws GmailReadTokenMissingError when no token row exists', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 200, body: FAKE_MESSAGE }));
    const client = new GmailClient({ fetchImpl });
    const tokenStore = new InMemoryTokenStore(cryptoConfig);
    // no seed → no token

    const exec = gmailReadExecutor({ client, tokenStore });
    await assert.rejects(
      () => exec({ message_id: 'msg-abc' }, CTX),
      (err: unknown) => err instanceof GmailReadTokenMissingError
    );
  });

  it('marks needs_reauth and re-throws on 401', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 401,
      body: { error: { code: 401, message: 'invalid token' } }
    }));
    const client = new GmailClient({ fetchImpl });
    const tokenStore = new InMemoryTokenStore(cryptoConfig);
    await seedToken(tokenStore, 'u-1', 'expired-token');

    const exec = gmailReadExecutor({ client, tokenStore });
    await assert.rejects(
      () => exec({ message_id: 'msg-abc' }, CTX),
      /Gmail returned 401/
    );

    const [view] = await tokenStore.list('u-1');
    assert.ok(view);
    assert.equal(view?.needs_reauth, true);
  });

  it('does NOT mark needs_reauth on 5xx (transient — could recover)', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 503,
      body: { error: { code: 503, message: 'temporarily unavailable' } }
    }));
    const client = new GmailClient({ fetchImpl });
    const tokenStore = new InMemoryTokenStore(cryptoConfig);
    await seedToken(tokenStore, 'u-1', 'good-token');

    const exec = gmailReadExecutor({ client, tokenStore });
    await assert.rejects(
      () => exec({ message_id: 'msg-abc' }, CTX),
      /Gmail API error \(503/
    );

    const [view] = await tokenStore.list('u-1');
    assert.equal(view?.needs_reauth, false);
  });
});

describe('gmailReadExecutor — per-user token isolation', () => {
  it('reads u-1 token even when u-2 has a different token', async () => {
    let lastAuth = '';
    const fetchImpl = mockFetch(async (_url, init) => {
      lastAuth = (init.headers as Record<string, string>).authorization;
      return { status: 200, body: FAKE_MESSAGE };
    });
    const client = new GmailClient({ fetchImpl });
    const tokenStore = new InMemoryTokenStore(cryptoConfig);
    await seedToken(tokenStore, 'u-1', 'token-u1');
    await seedToken(tokenStore, 'u-2', 'token-u2');

    const exec = gmailReadExecutor({ client, tokenStore });
    await exec({ message_id: 'msg-abc' }, { user_id: 'u-1', invocation_id: 'inv-1' });
    assert.equal(lastAuth, 'Bearer token-u1');

    await exec({ message_id: 'msg-abc' }, { user_id: 'u-2', invocation_id: 'inv-2' });
    assert.equal(lastAuth, 'Bearer token-u2');
  });
});

/* ---------------------------------------------------------------------- */
/* sendblue.send_user_message (Phase 3E.1)                                */
/* ---------------------------------------------------------------------- */

describe('sendBlueSendExecutor — happy path', () => {
  it('forwards to SendBlueClient.send and returns the outcome', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 200,
      body: { status: 'QUEUED', message_handle: 'h-1' }
    }));
    const client = new SendBlueClient({
      apiKeyId: 'k', apiSecretKey: 's', fetchImpl
    });
    const exec = sendBlueSendExecutor({ client });
    const out = await exec({ to: '+15555550100', content: 'hi' }, CTX);
    assert.equal(out.kind, 'sent');
    assert.equal(out.providerMessageHandle, 'h-1');
  });
});

describe('sendBlueSendExecutor — fail-closed when adapter not wired', () => {
  it('throws when client is undefined (defense-in-depth)', async () => {
    const exec = sendBlueSendExecutor({ client: undefined });
    await assert.rejects(
      () => exec({ to: '+15555550100', content: 'hi' }, CTX),
      /SendBlueClient not wired/
    );
  });
});

describe('sendBlueSendExecutor — input validation', () => {
  it('throws when args is missing', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 200, body: { status: 'QUEUED' } }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl });
    const exec = sendBlueSendExecutor({ client });
    await assert.rejects(
      () => exec(undefined as unknown as { to: string; content: string }, CTX),
      /args must be a SendInput/
    );
  });

  it('throws when args.to is missing or empty', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 200, body: { status: 'QUEUED' } }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl });
    const exec = sendBlueSendExecutor({ client });
    await assert.rejects(
      () => exec({ to: '', content: 'hi' }, CTX),
      /args\.to is required/
    );
  });

  it('throws when args.content is missing or empty', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 200, body: { status: 'QUEUED' } }));
    const client = new SendBlueClient({ apiKeyId: 'k', apiSecretKey: 's', fetchImpl });
    const exec = sendBlueSendExecutor({ client });
    await assert.rejects(
      () => exec({ to: '+15555550100', content: '' }, CTX),
      /args\.content is required/
    );
  });
});
