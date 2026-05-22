import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { CRYPTO_KEY_VERSION, decryptToken, encryptToken, loadCryptoConfig } from './token-crypto.ts';

const TEST_KEK = Buffer.alloc(32, 9).toString('base64');

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

describe('token-crypto.loadCryptoConfig', () => {
  it('refuses to boot without KEK in non-dev mode', () => {
    withEnv({ BREVIO_TOKEN_KEK: undefined, BREVIO_DEV_MODE: undefined }, () => {
      assert.throws(() => loadCryptoConfig(), /BREVIO_TOKEN_KEK required/);
    });
  });

  it('returns a per-process random KEK in dev mode', () => {
    withEnv({ BREVIO_TOKEN_KEK: undefined, BREVIO_DEV_MODE: 'true' }, () => {
      const c = loadCryptoConfig();
      assert.equal(c.devMode, true);
      assert.equal(c.kek?.length, 32);
    });
  });

  it('rejects KEK with wrong length', () => {
    withEnv({ BREVIO_TOKEN_KEK: Buffer.alloc(16, 1).toString('base64'), BREVIO_DEV_MODE: undefined }, () => {
      assert.throws(() => loadCryptoConfig(), /must decode to exactly 32 bytes/);
    });
  });
});

describe('token-crypto.encryptToken / decryptToken', () => {
  const config = withEnv({ BREVIO_TOKEN_KEK: TEST_KEK, BREVIO_DEV_MODE: undefined }, () => loadCryptoConfig());

  it('round-trips plaintext', () => {
    const enc = encryptToken(config, 'super-secret-token', 'user-1', 'google');
    assert.equal(enc.key_version, CRYPTO_KEY_VERSION);
    assert.ok(enc.ciphertext.length > 16);
    const dec = decryptToken(config, enc.ciphertext, enc.key_version, 'user-1', 'google');
    assert.equal(dec, 'super-secret-token');
  });

  it('produces different ciphertext for the same plaintext (random nonce)', () => {
    const a = encryptToken(config, 'x', 'u', 'p');
    const b = encryptToken(config, 'x', 'u', 'p');
    assert.notEqual(a.ciphertext.toString('hex'), b.ciphertext.toString('hex'));
  });

  it('rejects decryption with wrong AAD (wrong user_id)', () => {
    const enc = encryptToken(config, 'token', 'user-1', 'google');
    assert.throws(() => decryptToken(config, enc.ciphertext, enc.key_version, 'user-2', 'google'));
  });

  it('rejects decryption with wrong AAD (wrong provider)', () => {
    const enc = encryptToken(config, 'token', 'user-1', 'google');
    assert.throws(() => decryptToken(config, enc.ciphertext, enc.key_version, 'user-1', 'microsoft'));
  });

  it('rejects tampered ciphertext', () => {
    const enc = encryptToken(config, 'token', 'user-1', 'google');
    const tampered = Buffer.from(enc.ciphertext);
    tampered[14] ^= 0x42;
    assert.throws(() => decryptToken(config, tampered, enc.key_version, 'user-1', 'google'));
  });

  it('rejects too-short ciphertext', () => {
    assert.throws(() => decryptToken(config, Buffer.alloc(8), 1, 'u', 'p'), /too short/);
  });
});
