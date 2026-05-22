import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { OAuthError, exchangeCodeForToken, refreshAccessToken, revokeAtProvider } from './oauth-exchange.ts';
import type { ProviderConfig } from './oauth-providers/index.ts';

const config: ProviderConfig = {
  id: 'google',
  authorizeUrl: 'https://example.com/authorize',
  tokenUrl: 'https://example.com/token',
  revokeUrl: 'https://example.com/revoke',
  clientId: 'cid',
  clientSecret: 'sec',
  redirectUri: 'http://localhost/callback'
};

function mockFetch(handler: (url: string, init: RequestInit) => Promise<{ status: number; body: unknown; ok?: boolean }>): typeof fetch {
  return (async (url: string | URL, init?: RequestInit) => {
    const result = await handler(url.toString(), init ?? {});
    return new Response(typeof result.body === 'string' ? result.body : JSON.stringify(result.body), {
      status: result.status,
      headers: { 'content-type': 'application/json' }
    });
  }) as typeof fetch;
}

describe('exchangeCodeForToken', () => {
  it('returns token result on success', async () => {
    const fetchImpl = mockFetch(async (url, init) => {
      assert.equal(url, 'https://example.com/token');
      assert.equal(init.method, 'POST');
      assert.match(init.headers!['content-type'] as string, /application\/x-www-form-urlencoded/);
      const body = init.body as string;
      assert.match(body, /grant_type=authorization_code/);
      assert.match(body, /code=abc/);
      assert.match(body, /code_verifier=ver/);
      return {
        status: 200,
        body: { access_token: 'at_1', refresh_token: 'rt_1', token_type: 'Bearer', expires_in: 3600, scope: 'a b' }
      };
    });
    const result = await exchangeCodeForToken({ config, code: 'abc', codeVerifier: 'ver' }, fetchImpl);
    assert.equal(result.access_token, 'at_1');
    assert.equal(result.refresh_token, 'rt_1');
    assert.equal(result.expires_in, 3600);
  });

  it('throws OAuthError with retryable=false on invalid_grant', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 400,
      body: { error: 'invalid_grant', error_description: 'code expired' }
    }));
    await assert.rejects(
      exchangeCodeForToken({ config, code: 'x', codeVerifier: 'v' }, fetchImpl),
      (err: Error) => err instanceof OAuthError && err.providerError === 'invalid_grant' && err.retryable === false
    );
  });

  it('throws OAuthError with retryable=true on 5xx', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 503, body: { error: 'server_busy' } }));
    await assert.rejects(
      exchangeCodeForToken({ config, code: 'x', codeVerifier: 'v' }, fetchImpl),
      (err: Error) => err instanceof OAuthError && err.retryable === true
    );
  });

  it('throws OAuthError when access_token missing', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 200, body: { token_type: 'Bearer' } }));
    await assert.rejects(
      exchangeCodeForToken({ config, code: 'x', codeVerifier: 'v' }, fetchImpl),
      /missing access_token/
    );
  });
});

describe('refreshAccessToken', () => {
  it('round-trips a refresh', async () => {
    const fetchImpl = mockFetch(async (_url, init) => {
      const body = init.body as string;
      assert.match(body, /grant_type=refresh_token/);
      assert.match(body, /refresh_token=rt_old/);
      return { status: 200, body: { access_token: 'at_2', refresh_token: 'rt_new', token_type: 'Bearer', expires_in: 3600 } };
    });
    const result = await refreshAccessToken({ config, refreshToken: 'rt_old' }, fetchImpl);
    assert.equal(result.access_token, 'at_2');
    assert.equal(result.refresh_token, 'rt_new');
  });
});

describe('revokeAtProvider', () => {
  it('returns void on 200', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 200, body: {} }));
    await revokeAtProvider(config, 'tok', fetchImpl);
  });

  it('does not throw on 400 (token already invalid)', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 400, body: { error: 'invalid_token' } }));
    await revokeAtProvider(config, 'tok', fetchImpl);
  });

  it('throws on 500', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 500, body: { error: 'server' } }));
    await assert.rejects(revokeAtProvider(config, 'tok', fetchImpl), OAuthError);
  });

  it('does nothing if revokeUrl is undefined', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 500, body: {} }));
    await revokeAtProvider({ ...config, revokeUrl: undefined }, 'tok', fetchImpl);
  });
});
