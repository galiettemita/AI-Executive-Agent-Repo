import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { loadCryptoConfig } from './crypto.ts';
import { InMemoryTokenStore } from './token-store.ts';

const TEST_KEK = Buffer.alloc(32, 5).toString('base64');

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

const cryptoConfig = withEnv({ BREVIO_TOKEN_KEK: TEST_KEK, BREVIO_DEV_MODE: undefined }, () => loadCryptoConfig());

describe('InMemoryTokenStore', () => {
  it('saves and reads back access + refresh tokens (round-trip via decrypt)', async () => {
    const store = new InMemoryTokenStore(cryptoConfig);
    await store.save({
      user_id: 'u1',
      provider: 'google',
      scopes: ['scope_a', 'scope_b'],
      access_token: 'at_abc',
      refresh_token: 'rt_xyz',
      expires_at: new Date(Date.now() + 3600_000)
    });
    assert.equal(await store.loadAccessToken('u1', 'google'), 'at_abc');
    assert.equal(await store.loadRefreshToken('u1', 'google'), 'rt_xyz');
  });

  it('returns null for missing rows', async () => {
    const store = new InMemoryTokenStore(cryptoConfig);
    assert.equal(await store.loadAccessToken('u-none', 'google'), null);
    assert.equal(await store.loadRefreshToken('u-none', 'google'), null);
  });

  it('handles missing refresh token', async () => {
    const store = new InMemoryTokenStore(cryptoConfig);
    await store.save({ user_id: 'u1', provider: 'google', scopes: [], access_token: 'at' });
    assert.equal(await store.loadRefreshToken('u1', 'google'), null);
  });

  it('list returns user-scoped views without ciphertext', async () => {
    const store = new InMemoryTokenStore(cryptoConfig);
    await store.save({ user_id: 'u1', provider: 'google', scopes: ['s'], access_token: 'a' });
    await store.save({ user_id: 'u1', provider: 'spotify', scopes: ['p'], access_token: 'b' });
    await store.save({ user_id: 'u2', provider: 'google', scopes: ['s'], access_token: 'c' });
    const u1 = await store.list('u1');
    assert.equal(u1.length, 2);
    assert.deepEqual(u1.map((v) => v.provider).sort(), ['google', 'spotify']);
    // No ciphertext leakage
    for (const v of u1) {
      assert.ok(!('access_token_ciphertext' in v));
    }
  });

  it('delete is idempotent', async () => {
    const store = new InMemoryTokenStore(cryptoConfig);
    await store.save({ user_id: 'u1', provider: 'google', scopes: [], access_token: 'a' });
    await store.delete('u1', 'google');
    await store.delete('u1', 'google');
    assert.equal(await store.has('u1', 'google'), false);
  });

  it('markNeedsReauth is reflected in list()', async () => {
    const store = new InMemoryTokenStore(cryptoConfig);
    await store.save({ user_id: 'u1', provider: 'google', scopes: [], access_token: 'a' });
    await store.markNeedsReauth('u1', 'google');
    const list = await store.list('u1');
    assert.equal(list[0]?.needs_reauth, true);
  });
});
