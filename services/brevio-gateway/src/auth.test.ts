import assert from 'node:assert/strict';
import { afterEach, beforeEach, describe, it } from 'node:test';

import {
  extractBearerToken,
  extractCookieToken,
  loadAuthConfig,
  signSessionToken,
  verifySessionToken
} from './auth.ts';

const TEST_KEY = Buffer.alloc(32, 7).toString('base64');

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

describe('auth.loadAuthConfig', () => {
  it('refuses to boot without signing key in non-dev mode', () => {
    withEnv({ BREVIO_SESSION_SIGNING_KEY: undefined, BREVIO_DEV_MODE: undefined }, () => {
      assert.throws(() => loadAuthConfig(), /BREVIO_SESSION_SIGNING_KEY required/);
    });
  });

  it('boots with no signing key when BREVIO_DEV_MODE=true', () => {
    withEnv({ BREVIO_SESSION_SIGNING_KEY: undefined, BREVIO_DEV_MODE: 'true' }, () => {
      const config = loadAuthConfig();
      assert.equal(config.devMode, true);
      assert.equal(config.signingKey, undefined);
    });
  });

  it('boots with valid signing key', () => {
    withEnv({ BREVIO_SESSION_SIGNING_KEY: TEST_KEY, BREVIO_DEV_MODE: undefined }, () => {
      const config = loadAuthConfig();
      assert.equal(config.devMode, false);
      assert.ok(config.signingKey);
      assert.equal(config.signingKey?.length, 32);
    });
  });

  it('rejects too-short signing key', () => {
    withEnv({ BREVIO_SESSION_SIGNING_KEY: Buffer.alloc(16, 1).toString('base64'), BREVIO_DEV_MODE: undefined }, () => {
      assert.throws(() => loadAuthConfig(), /at least 32 bytes/);
    });
  });
});

describe('auth.signSessionToken / verifySessionToken', () => {
  it('round-trips a valid token', () => {
    const config = withEnv({ BREVIO_SESSION_SIGNING_KEY: TEST_KEY, BREVIO_DEV_MODE: undefined }, () => loadAuthConfig());
    const token = signSessionToken(config, {
      user_id: 'u1',
      session_id: 's1',
      expires_at: Math.floor(Date.now() / 1000) + 3600
    });
    const verified = verifySessionToken(config, token);
    assert.ok(verified);
    assert.equal(verified?.user_id, 'u1');
    assert.equal(verified?.session_id, 's1');
  });

  it('rejects expired token', () => {
    const config = withEnv({ BREVIO_SESSION_SIGNING_KEY: TEST_KEY, BREVIO_DEV_MODE: undefined }, () => loadAuthConfig());
    const token = signSessionToken(config, {
      user_id: 'u1',
      session_id: 's1',
      expires_at: Math.floor(Date.now() / 1000) - 1
    });
    assert.equal(verifySessionToken(config, token), null);
  });

  it('rejects forged HMAC', () => {
    const config = withEnv({ BREVIO_SESSION_SIGNING_KEY: TEST_KEY, BREVIO_DEV_MODE: undefined }, () => loadAuthConfig());
    const token = signSessionToken(config, {
      user_id: 'u1',
      session_id: 's1',
      expires_at: Math.floor(Date.now() / 1000) + 3600
    });
    const tampered = token.slice(0, -4) + 'AAAA';
    assert.equal(verifySessionToken(config, tampered), null);
  });

  it('rejects tampered payload', () => {
    const config = withEnv({ BREVIO_SESSION_SIGNING_KEY: TEST_KEY, BREVIO_DEV_MODE: undefined }, () => loadAuthConfig());
    const token = signSessionToken(config, {
      user_id: 'u1',
      session_id: 's1',
      expires_at: Math.floor(Date.now() / 1000) + 3600
    });
    const dot = token.indexOf('.');
    const tampered = token.slice(0, dot - 4) + 'AAAA' + token.slice(dot);
    assert.equal(verifySessionToken(config, tampered), null);
  });

  it('rejects empty/missing token', () => {
    const config = withEnv({ BREVIO_SESSION_SIGNING_KEY: TEST_KEY, BREVIO_DEV_MODE: undefined }, () => loadAuthConfig());
    assert.equal(verifySessionToken(config, undefined), null);
    assert.equal(verifySessionToken(config, ''), null);
    assert.equal(verifySessionToken(config, 'no-dot'), null);
  });
});

describe('extractBearerToken / extractCookieToken', () => {
  it('extracts bearer', () => {
    assert.equal(extractBearerToken('Bearer abc.def'), 'abc.def');
    assert.equal(extractBearerToken('bearer abc.def'), 'abc.def');
  });
  it('returns undefined for non-bearer', () => {
    assert.equal(extractBearerToken('Basic xyz'), undefined);
    assert.equal(extractBearerToken(undefined), undefined);
  });
  it('extracts named cookie', () => {
    assert.equal(extractCookieToken('foo=bar; brevio_session=tok.sig; baz=qux'), 'tok.sig');
    assert.equal(extractCookieToken('brevio_session=tok'), 'tok');
    assert.equal(extractCookieToken('other=x'), undefined);
  });
});
