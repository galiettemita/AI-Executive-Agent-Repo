import assert from 'node:assert/strict';
import http from 'node:http';
import { describe, it } from 'node:test';

import { InMemoryAuditStore } from './audit.ts';
import { InMemoryConsentStore } from './consent-store.ts';
import { loadCryptoConfig } from './crypto.ts';
import { handleOAuthCallback, handleOAuthStart } from './oauth-routes.ts';
import { InMemoryNonceStore, generateNonce, generatePKCEVerifier } from './oauth-state.ts';
import { loadOAuthStateConfig, buildState } from './oauth-state.ts';
import { InMemoryTokenStore } from './token-store.ts';

const TEST_KEK = Buffer.alloc(32, 1).toString('base64');
const TEST_STATE_KEY = Buffer.alloc(32, 2).toString('base64');

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

class MockResponse {
  statusCode = 200;
  headers: Record<string, string> = {};
  body = '';
  writeHead(status: number, headers: Record<string, string> = {}): this {
    this.statusCode = status;
    Object.assign(this.headers, headers);
    return this;
  }
  setHeader(name: string, value: string | number): void {
    this.headers[name] = String(value);
  }
  end(body?: string): void {
    if (body) this.body += body;
  }
}

function makeReq(): http.IncomingMessage {
  const r = new http.IncomingMessage(null as never);
  return r;
}

describe('isRedirectAllowed via callback (origin-exact match)', () => {
  it('rejects subdomain look-alike (CRITICAL: open-redirect regression)', async () => {
    return withEnv({
      BREVIO_OAUTH_POST_REDIRECT_ALLOWED: 'http://localhost:3333',
      GOOGLE_CLIENT_ID: 'cid',
      GOOGLE_CLIENT_SECRET: 'sec',
      BREVIO_OAUTH_REDIRECT_URI_GOOGLE: 'http://localhost:3333/api/v1/oauth/callback',
      BREVIO_OAUTH_STATE_KEY: TEST_STATE_KEY,
      BREVIO_TOKEN_KEK: TEST_KEK,
      BREVIO_DEV_MODE: undefined
    }, async () => {
      const stateConfig = loadOAuthStateConfig();
      const cryptoConfig = loadCryptoConfig();
      const nonceStore = new InMemoryNonceStore();
      const consentStore = new InMemoryConsentStore();
      const tokenStore = new InMemoryTokenStore(cryptoConfig);
      const auditStore = new InMemoryAuditStore();

      const verifier = generatePKCEVerifier();
      const nonce = generateNonce();
      await nonceStore.put({
        nonce, user_id: 'u1', provider: 'google', skill_id: 'google-calendar',
        code_verifier: verifier, pending_message_id: null, created_at: Date.now(), consumed: false
      });
      const state = buildState(stateConfig, {
        user_id: 'u1', provider: 'google', skill_id: 'google-calendar',
        pending_message_id: null, iat: Date.now(), nonce
      });

      const url = new URL(`http://localhost/api/v1/oauth/callback?code=X&state=${encodeURIComponent(state)}&post_redirect=${encodeURIComponent('http://localhost:3333.evil.com/leak')}&error=user_denied`);
      const res = new MockResponse();
      const fakeFetch = (async () => new Response('{}', { status: 200 })) as typeof fetch;

      await handleOAuthCallback(makeReq(), res as unknown as http.ServerResponse, {
        auth: { user_id: 'u1', session_id: 's1', source: 'session' },
        ip: null, userAgent: null, requestId: 'r1', url
      }, { consentStore, tokenStore, auditStore, nonceStore, stateConfig, fetchImpl: fakeFetch });

      // Error param triggers redirect — but it must NOT go to evil.com
      assert.equal(res.statusCode, 302);
      const location = res.headers.location ?? '';
      assert.ok(!location.includes('evil.com'), `redirect leaked to evil.com: ${location}`);
      assert.ok(location.startsWith('http://localhost:3333'), `expected fallback to default allowed, got: ${location}`);
    });
  });

  it('rejects wrong scheme', async () => {
    return withEnv({
      BREVIO_OAUTH_POST_REDIRECT_ALLOWED: 'http://localhost:3333',
      GOOGLE_CLIENT_ID: 'cid',
      GOOGLE_CLIENT_SECRET: 'sec',
      BREVIO_OAUTH_REDIRECT_URI_GOOGLE: 'http://localhost:3333/api/v1/oauth/callback',
      BREVIO_OAUTH_STATE_KEY: TEST_STATE_KEY,
      BREVIO_TOKEN_KEK: TEST_KEK,
      BREVIO_DEV_MODE: undefined
    }, async () => {
      const stateConfig = loadOAuthStateConfig();
      const cryptoConfig = loadCryptoConfig();
      const nonceStore = new InMemoryNonceStore();
      const consentStore = new InMemoryConsentStore();
      const tokenStore = new InMemoryTokenStore(cryptoConfig);
      const auditStore = new InMemoryAuditStore();

      const url = new URL('http://localhost/api/v1/oauth/callback?error=user_denied&post_redirect=https%3A%2F%2Flocalhost%3A3333');
      const res = new MockResponse();
      const fakeFetch = (async () => new Response('{}', { status: 200 })) as typeof fetch;

      await handleOAuthCallback(makeReq(), res as unknown as http.ServerResponse, {
        auth: { user_id: 'u1', session_id: 's1', source: 'session' },
        ip: null, userAgent: null, requestId: 'r1', url
      }, { consentStore, tokenStore, auditStore, nonceStore, stateConfig, fetchImpl: fakeFetch });

      assert.equal(res.statusCode, 302);
      const location = res.headers.location ?? '';
      assert.ok(!location.startsWith('https://'), `redirect leaked to wrong-scheme URL: ${location}`);
    });
  });
});

describe('handleOAuthStart input validation', () => {
  const env = {
    BREVIO_OAUTH_POST_REDIRECT_ALLOWED: 'http://localhost:3333',
    GOOGLE_CLIENT_ID: 'cid',
    GOOGLE_CLIENT_SECRET: 'sec',
    BREVIO_OAUTH_REDIRECT_URI_GOOGLE: 'http://localhost:3333/api/v1/oauth/callback',
    BREVIO_OAUTH_STATE_KEY: TEST_STATE_KEY,
    BREVIO_TOKEN_KEK: TEST_KEK,
    BREVIO_DEV_MODE: undefined
  };

  it('rejects unsupported provider', async () => {
    return withEnv(env, async () => {
      const stateConfig = loadOAuthStateConfig();
      const cryptoConfig = loadCryptoConfig();
      const url = new URL('http://localhost/api/v1/oauth/start?provider=hacker&skill_id=google-calendar');
      const res = new MockResponse();
      await handleOAuthStart(makeReq(), res as unknown as http.ServerResponse, {
        auth: { user_id: 'u1', session_id: 's1', source: 'session' },
        ip: null, userAgent: null, requestId: 'r1', url
      }, {
        consentStore: new InMemoryConsentStore(),
        tokenStore: new InMemoryTokenStore(cryptoConfig),
        auditStore: new InMemoryAuditStore(),
        nonceStore: new InMemoryNonceStore(),
        stateConfig
      });
      assert.equal(res.statusCode, 400);
      assert.match(res.body, /invalid_provider/);
    });
  });

  it('rejects skill_id with weird chars', async () => {
    return withEnv(env, async () => {
      const stateConfig = loadOAuthStateConfig();
      const cryptoConfig = loadCryptoConfig();
      const url = new URL('http://localhost/api/v1/oauth/start?provider=google&skill_id=evil%3B--');
      const res = new MockResponse();
      await handleOAuthStart(makeReq(), res as unknown as http.ServerResponse, {
        auth: { user_id: 'u1', session_id: 's1', source: 'session' },
        ip: null, userAgent: null, requestId: 'r1', url
      }, {
        consentStore: new InMemoryConsentStore(),
        tokenStore: new InMemoryTokenStore(cryptoConfig),
        auditStore: new InMemoryAuditStore(),
        nonceStore: new InMemoryNonceStore(),
        stateConfig
      });
      assert.equal(res.statusCode, 400);
      assert.match(res.body, /invalid_skill_id/);
    });
  });

  it('rejects mismatched provider/skill (skill belongs to different provider)', async () => {
    return withEnv(env, async () => {
      const stateConfig = loadOAuthStateConfig();
      const cryptoConfig = loadCryptoConfig();
      const url = new URL('http://localhost/api/v1/oauth/start?provider=google&skill_id=outlook');
      const res = new MockResponse();
      await handleOAuthStart(makeReq(), res as unknown as http.ServerResponse, {
        auth: { user_id: 'u1', session_id: 's1', source: 'session' },
        ip: null, userAgent: null, requestId: 'r1', url
      }, {
        consentStore: new InMemoryConsentStore(),
        tokenStore: new InMemoryTokenStore(cryptoConfig),
        auditStore: new InMemoryAuditStore(),
        nonceStore: new InMemoryNonceStore(),
        stateConfig
      });
      assert.equal(res.statusCode, 400);
      assert.match(res.body, /provider_skill_mismatch/);
    });
  });

  it('builds 302 with PKCE challenge when valid', async () => {
    return withEnv(env, async () => {
      const stateConfig = loadOAuthStateConfig();
      const cryptoConfig = loadCryptoConfig();
      const url = new URL('http://localhost/api/v1/oauth/start?provider=google&skill_id=google-calendar');
      const res = new MockResponse();
      await handleOAuthStart(makeReq(), res as unknown as http.ServerResponse, {
        auth: { user_id: 'u1', session_id: 's1', source: 'session' },
        ip: null, userAgent: null, requestId: 'r1', url
      }, {
        consentStore: new InMemoryConsentStore(),
        tokenStore: new InMemoryTokenStore(cryptoConfig),
        auditStore: new InMemoryAuditStore(),
        nonceStore: new InMemoryNonceStore(),
        stateConfig
      });
      assert.equal(res.statusCode, 302);
      const loc = res.headers.location ?? '';
      assert.match(loc, /accounts\.google\.com/);
      assert.match(loc, /code_challenge=/);
      assert.match(loc, /code_challenge_method=S256/);
      assert.match(loc, /state=/);
      assert.match(loc, /scope=https/);
    });
  });
});
