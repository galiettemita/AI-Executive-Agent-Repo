import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryGmailCursorStore } from '../memory/gmail-cursors.ts';
import { GmailClient, GmailUnauthorizedError } from '../adapters/gmail/client.ts';
import { loadCryptoConfig } from '../security/token-crypto.ts';
import { InMemoryTokenStore } from '../security/oauth/token-store.ts';
import {
  InMemoryNonceStore,
  generateNonce,
  loadOAuthStateConfig
} from '../security/oauth/state.ts';
import { type ProviderConfig } from '../security/oauth/providers/index.ts';

import {
  type OAuthGoogleRouteDeps,
  handleOAuthGoogleCallback,
  handleOAuthGoogleStart
} from './oauth-google.ts';

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

const TEST_KEK = Buffer.alloc(32, 9).toString('base64');
const TEST_OAUTH_KEY = Buffer.alloc(32, 11).toString('base64');

function makeDeps(overrides: Partial<OAuthGoogleRouteDeps> = {}): OAuthGoogleRouteDeps {
  const providerConfig: ProviderConfig = {
    id: 'google',
    authorizeUrl: 'https://accounts.google.test/auth',
    tokenUrl: 'https://oauth.googleapis.test/token',
    revokeUrl: 'https://oauth.googleapis.test/revoke',
    clientId: 'test-client-id',
    clientSecret: 'test-client-secret',
    redirectUri: 'https://app.brevio.test/oauth/google/callback',
    extraAuthorizeParams: { access_type: 'offline', prompt: 'consent' }
  };
  const stateConfig = withEnv(
    { BREVIO_OAUTH_STATE_KEY: TEST_OAUTH_KEY, BREVIO_DEV_MODE: undefined },
    () => loadOAuthStateConfig()
  );
  const cryptoConfig = withEnv(
    { BREVIO_TOKEN_KEK: TEST_KEK, BREVIO_DEV_MODE: undefined },
    () => loadCryptoConfig()
  );
  return {
    providerConfig,
    stateConfig,
    nonceStore: new InMemoryNonceStore(),
    tokenStore: new InMemoryTokenStore(cryptoConfig),
    gmailCursorStore: new InMemoryGmailCursorStore(),
    gmailClient: new GmailClient({
      fetchImpl: (async () =>
        new Response(JSON.stringify({}), { status: 500 })) as typeof fetch
    }),
    ...overrides
  };
}

/* ---------------------------------------------------------------------- */
/* /start                                                                 */
/* ---------------------------------------------------------------------- */

describe('handleOAuthGoogleStart', () => {
  it('returns an authorize URL with gmail.readonly scope and stores a nonce', async () => {
    const deps = makeDeps();
    const result = await handleOAuthGoogleStart({ user_id: 'u-1' }, deps);
    assert.ok(result.authorize_url.startsWith('https://accounts.google.test/auth?'));
    const url = new URL(result.authorize_url);
    assert.equal(url.searchParams.get('client_id'), 'test-client-id');
    assert.equal(url.searchParams.get('redirect_uri'), 'https://app.brevio.test/oauth/google/callback');
    assert.equal(url.searchParams.get('response_type'), 'code');
    assert.equal(
      url.searchParams.get('scope'),
      'https://www.googleapis.com/auth/gmail.readonly'
    );
    assert.equal(url.searchParams.get('code_challenge_method'), 'S256');
    assert.ok(url.searchParams.get('code_challenge'));
    assert.equal(url.searchParams.get('access_type'), 'offline');
    assert.equal(url.searchParams.get('prompt'), 'consent');
    // The state we returned should match what's in the authorize URL.
    assert.equal(url.searchParams.get('state'), result.state);
    // Nonce was stored — consume should return the row once.
    const consumed = await deps.nonceStore.consume(result.nonce);
    assert.ok(consumed);
    assert.equal(consumed?.user_id, 'u-1');
    assert.equal(consumed?.provider, 'google');
  });

  it('two consecutive /start calls produce different nonces + states (no replay)', async () => {
    const deps = makeDeps();
    const a = await handleOAuthGoogleStart({ user_id: 'u-1' }, deps);
    const b = await handleOAuthGoogleStart({ user_id: 'u-1' }, deps);
    assert.notEqual(a.nonce, b.nonce);
    assert.notEqual(a.state, b.state);
  });
});

/* ---------------------------------------------------------------------- */
/* /callback                                                              */
/* ---------------------------------------------------------------------- */

function mockTokenFetch(
  handler: (init: RequestInit) => { status: number; body: unknown }
): typeof fetch {
  return (async (_url: string | URL | Request, init?: RequestInit) => {
    const result = handler(init ?? {});
    return new Response(JSON.stringify(result.body), {
      status: result.status,
      headers: { 'content-type': 'application/json' }
    });
  }) as typeof fetch;
}

function mockGmailProfileClient(
  profile: { emailAddress: string; historyId: string; messagesTotal?: number; threadsTotal?: number },
  throwError?: Error
): GmailClient {
  const fetchImpl = (async () => {
    if (throwError) throw throwError;
    return new Response(JSON.stringify(profile), {
      status: 200,
      headers: { 'content-type': 'application/json' }
    });
  }) as typeof fetch;
  return new GmailClient({ fetchImpl });
}

describe('handleOAuthGoogleCallback — happy path', () => {
  it('exchanges code, stores tokens, seeds Gmail cursor', async () => {
    const deps = makeDeps({
      fetchImpl: mockTokenFetch(() => ({
        status: 200,
        body: {
          access_token: 'at-secret',
          refresh_token: 'rt-secret',
          token_type: 'Bearer',
          expires_in: 3600,
          scope: 'https://www.googleapis.com/auth/gmail.readonly'
        }
      })),
      gmailClient: mockGmailProfileClient({
        emailAddress: 'founder@example.com',
        historyId: '99001'
      })
    });
    const start = await handleOAuthGoogleStart({ user_id: 'u-callback-1' }, deps);
    const result = await handleOAuthGoogleCallback(
      { code: 'auth-code-from-google', state: start.state },
      deps
    );

    assert.equal(result.ok, true);
    if (result.ok) {
      assert.equal(result.user_id, 'u-callback-1');
      assert.equal(result.provider, 'google');
      assert.equal(result.email_address, 'founder@example.com');
      assert.equal(result.gmail_history_id, '99001');
    }
    // Tokens persisted (round-trip via decrypt).
    assert.equal(await deps.tokenStore.loadAccessToken('u-callback-1', 'google'), 'at-secret');
    assert.equal(await deps.tokenStore.loadRefreshToken('u-callback-1', 'google'), 'rt-secret');
    // Gmail cursor seeded.
    const cursor = await deps.gmailCursorStore.get('u-callback-1');
    assert.equal(cursor?.history_id, '99001');
  });
});

describe('handleOAuthGoogleCallback — failure paths', () => {
  it('missing_code_or_state when code is empty', async () => {
    const deps = makeDeps();
    const start = await handleOAuthGoogleStart({ user_id: 'u-2' }, deps);
    const result = await handleOAuthGoogleCallback({ code: '', state: start.state }, deps);
    assert.equal(result.ok, false);
    if (!result.ok) assert.equal(result.code, 'missing_code_or_state');
  });

  it('missing_code_or_state when state is empty', async () => {
    const deps = makeDeps();
    const result = await handleOAuthGoogleCallback({ code: 'auth-code', state: '' }, deps);
    assert.equal(result.ok, false);
    if (!result.ok) assert.equal(result.code, 'missing_code_or_state');
  });

  it('invalid_state when state HMAC fails', async () => {
    const deps = makeDeps();
    const result = await handleOAuthGoogleCallback(
      { code: 'auth-code', state: 'eyJ0YW1wZXJlZCI6dHJ1ZX0.bogus' },
      deps
    );
    assert.equal(result.ok, false);
    if (!result.ok) assert.equal(result.code, 'invalid_state');
  });

  it('nonce_consumed_or_unknown on second use of the same state (replay protection)', async () => {
    const deps = makeDeps({
      fetchImpl: mockTokenFetch(() => ({
        status: 200,
        body: {
          access_token: 'at', refresh_token: 'rt', token_type: 'Bearer', expires_in: 3600
        }
      })),
      gmailClient: mockGmailProfileClient({ emailAddress: 'u@x.com', historyId: '1' })
    });
    const start = await handleOAuthGoogleStart({ user_id: 'u-replay' }, deps);
    const first = await handleOAuthGoogleCallback({ code: 'code-1', state: start.state }, deps);
    assert.equal(first.ok, true);
    const second = await handleOAuthGoogleCallback({ code: 'code-1', state: start.state }, deps);
    assert.equal(second.ok, false);
    if (!second.ok) assert.equal(second.code, 'nonce_consumed_or_unknown');
  });

  it('exchange_failed when the OAuth token exchange returns an error', async () => {
    const deps = makeDeps({
      fetchImpl: mockTokenFetch(() => ({
        status: 400,
        body: { error: 'invalid_grant', error_description: 'code expired' }
      }))
    });
    const start = await handleOAuthGoogleStart({ user_id: 'u-exch' }, deps);
    const result = await handleOAuthGoogleCallback({ code: 'expired-code', state: start.state }, deps);
    assert.equal(result.ok, false);
    if (!result.ok) {
      assert.equal(result.code, 'exchange_failed');
      assert.match(result.reason, /invalid_grant/);
    }
    // Nothing should be persisted on exchange failure.
    assert.equal(await deps.tokenStore.has('u-exch', 'google'), false);
    assert.equal(await deps.gmailCursorStore.get('u-exch'), null);
  });

  it('gmail_profile_failed when Gmail rejects the freshly-issued access token', async () => {
    const deps = makeDeps({
      fetchImpl: mockTokenFetch(() => ({
        status: 200,
        body: { access_token: 'at', refresh_token: 'rt', token_type: 'Bearer', expires_in: 3600 }
      })),
      gmailClient: mockGmailProfileClient(
        { emailAddress: '', historyId: '' },
        new GmailUnauthorizedError('test rejection')
      )
    });
    const start = await handleOAuthGoogleStart({ user_id: 'u-gmail-fail' }, deps);
    const result = await handleOAuthGoogleCallback({ code: 'code', state: start.state }, deps);
    assert.equal(result.ok, false);
    if (!result.ok) assert.equal(result.code, 'gmail_profile_failed');
    // Note: token IS persisted before the profile fetch — that's by
    // design so the user does not have to re-do the OAuth flow if Gmail
    // glitches transiently. The cursor is NOT initialized.
    assert.equal(await deps.tokenStore.has('u-gmail-fail', 'google'), true);
    assert.equal(await deps.gmailCursorStore.get('u-gmail-fail'), null);
  });
});

describe('handleOAuthGoogleCallback — defense-in-depth', () => {
  it('state_user_mismatch — synthesized inconsistent nonce + state', async () => {
    // Construct a deps where the nonce row's user_id differs from the
    // state claims' user_id. Use the InMemoryNonceStore's put() directly
    // with a mismatched user_id, then build a state that points at the
    // same nonce but a different user.
    const deps = makeDeps();
    const nonceValue = generateNonce();
    const start = await handleOAuthGoogleStart({ user_id: 'u-real' }, deps);
    // Hijack: overwrite the just-issued nonce row with a different
    // user_id (simulates an attacker who got a valid state but the
    // nonce row was corrupted/swapped).
    await deps.nonceStore.put({
      nonce: nonceValue,
      user_id: 'u-attacker',
      provider: 'google',
      skill_id: 'gmail.read',
      code_verifier: 'v',
      pending_message_id: null,
      created_at: Date.now(),
      consumed: false
    });
    // Build a state whose nonce claims to be nonceValue.
    const { buildState } = await import('../security/oauth/state.ts');
    const forgedState = buildState(deps.stateConfig, {
      user_id: 'u-real',
      provider: 'google',
      skill_id: 'gmail.read',
      pending_message_id: null,
      iat: Date.now(),
      nonce: nonceValue
    });
    const result = await handleOAuthGoogleCallback({ code: 'code', state: forgedState }, deps);
    assert.equal(result.ok, false);
    if (!result.ok) assert.equal(result.code, 'state_user_mismatch');
    // start variable used to make sure compiler doesn't complain
    void start;
  });
});
