import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  InMemoryNonceStore,
  STATE_TTL_MS_EXPORT,
  buildState,
  deriveCodeChallenge,
  generateNonce,
  generatePKCEVerifier,
  loadOAuthStateConfig,
  verifyState
} from './state.ts';

const TEST_KEY = Buffer.alloc(32, 3).toString('base64');

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

describe('oauth-state.loadOAuthStateConfig', () => {
  it('refuses without key in non-dev mode', () => {
    withEnv({ BREVIO_OAUTH_STATE_KEY: undefined, BREVIO_DEV_MODE: undefined }, () => {
      assert.throws(() => loadOAuthStateConfig(), /BREVIO_OAUTH_STATE_KEY required/);
    });
  });

  it('rejects too-short key', () => {
    withEnv({ BREVIO_OAUTH_STATE_KEY: Buffer.alloc(16, 1).toString('base64'), BREVIO_DEV_MODE: undefined }, () => {
      assert.throws(() => loadOAuthStateConfig(), /at least 32 bytes/);
    });
  });

  it('uses random key in dev', () => {
    withEnv({ BREVIO_OAUTH_STATE_KEY: undefined, BREVIO_DEV_MODE: 'true' }, () => {
      const c = loadOAuthStateConfig();
      assert.equal(c.signingKey.length, 32);
    });
  });
});

describe('PKCE helpers', () => {
  it('verifier is 32+ chars URL-safe', () => {
    const v = generatePKCEVerifier();
    assert.match(v, /^[A-Za-z0-9_-]+$/);
    assert.ok(v.length >= 32);
  });

  it('challenge is deterministic for a given verifier', () => {
    const v = 'verifier-string-of-some-length';
    const c1 = deriveCodeChallenge(v);
    const c2 = deriveCodeChallenge(v);
    assert.equal(c1, c2);
  });

  it('different verifiers yield different challenges', () => {
    assert.notEqual(deriveCodeChallenge('a'), deriveCodeChallenge('b'));
  });
});

describe('state HMAC', () => {
  const config = withEnv({ BREVIO_OAUTH_STATE_KEY: TEST_KEY, BREVIO_DEV_MODE: undefined }, () => loadOAuthStateConfig());

  it('round-trips claims', () => {
    const claims = {
      user_id: 'u1',
      provider: 'google',
      skill_id: 'google-calendar',
      pending_message_id: null,
      iat: Date.now(),
      nonce: generateNonce()
    };
    const state = buildState(config, claims);
    const verified = verifyState(config, state, Date.now());
    assert.ok(verified);
    assert.equal(verified?.user_id, 'u1');
    assert.equal(verified?.skill_id, 'google-calendar');
  });

  it('rejects expired state', () => {
    const claims = {
      user_id: 'u1',
      provider: 'google',
      skill_id: 's',
      pending_message_id: null,
      iat: Date.now() - STATE_TTL_MS_EXPORT - 1000,
      nonce: 'n'
    };
    const state = buildState(config, claims);
    assert.equal(verifyState(config, state, Date.now()), null);
  });

  it('rejects tampered HMAC', () => {
    const state = buildState(config, {
      user_id: 'u1', provider: 'p', skill_id: 's', pending_message_id: null, iat: Date.now(), nonce: 'n'
    });
    const tampered = state.slice(0, -4) + 'AAAA';
    assert.equal(verifyState(config, tampered, Date.now()), null);
  });

  it('rejects tampered payload', () => {
    const state = buildState(config, {
      user_id: 'u1', provider: 'p', skill_id: 's', pending_message_id: null, iat: Date.now(), nonce: 'n'
    });
    const dot = state.indexOf('.');
    const tampered = state.slice(0, dot - 4) + 'AAAA' + state.slice(dot);
    assert.equal(verifyState(config, tampered, Date.now()), null);
  });
});

describe('InMemoryNonceStore', () => {
  it('consume returns row once then null (replay protection)', async () => {
    const store = new InMemoryNonceStore();
    await store.put({
      nonce: 'n1', user_id: 'u1', provider: 'google', skill_id: 's',
      code_verifier: 'v', pending_message_id: null, created_at: Date.now(), consumed: false
    });
    const first = await store.consume('n1');
    assert.ok(first);
    const second = await store.consume('n1');
    assert.equal(second, null);
  });

  it('prune removes stale rows', async () => {
    const store = new InMemoryNonceStore();
    const now = Date.now();
    await store.put({
      nonce: 'old', user_id: 'u', provider: 'p', skill_id: 's',
      code_verifier: 'v', pending_message_id: null, created_at: now - STATE_TTL_MS_EXPORT - 1000, consumed: false
    });
    await store.put({
      nonce: 'fresh', user_id: 'u', provider: 'p', skill_id: 's',
      code_verifier: 'v', pending_message_id: null, created_at: now, consumed: false
    });
    const pruned = await store.prune(now);
    assert.equal(pruned, 1);
    assert.ok(await store.consume('fresh'));
  });
});
