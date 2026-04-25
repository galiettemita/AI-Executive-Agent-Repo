import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { signAccessToken, signCallerContextEnvelope } from '../../../packages/shared/src/security.js';
import { loadAuthServiceMap } from './config.js';
import { createAuthServiceRuntime } from './server.js';
import type { EnvConfig } from './types.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, '..', '..', '..');
const mapPath = path.join(repoRoot, 'config', 'auth-service-map.yaml');
const internalAuthSecret = 'auth-test-secret';
const callerContextSecret = 'auth-caller-secret';

function serviceToken(audience: string, tokenUse: 'service_access' | 'admin_access' = 'service_access'): string {
  return signAccessToken(internalAuthSecret, {
    version: 2,
    sub: tokenUse === 'admin_access' ? 'admin-user' : 'brevio-gateway',
    iss: 'https://auth.brevio.internal',
    aud: audience,
    iat: Math.floor(Date.now() / 1000),
    exp: Math.floor(Date.now() / 1000) + 300,
    token_use: tokenUse,
    scopes: ['oauth:test']
  });
}

function callerContext(userId = 'u-test-1'): string {
  return signCallerContextEnvelope(callerContextSecret, {
    subject: userId,
    user_id: userId,
    auth_strength: 'service_token',
    provenance: 'test',
    issued_at: new Date().toISOString(),
    expires_at: new Date(Date.now() + 300000).toISOString()
  });
}

async function startRuntime() {
  const map = loadAuthServiceMap(mapPath);
  const config: EnvConfig = {
    serviceName: 'brevio-auth',
    serviceVersion: 'test',
    environment: 'test',
    port: 0,
    mapPath,
    internalAuthSecret,
    internalAuthIssuer: 'https://auth.brevio.internal',
    serviceAudience: 'brevio-auth',
    callerContextSecret,
    logSalt: 'auth-test-salt',
    stateEncryptionSecret: 'auth-state-secret',
    completionRedirectAllowlist: {
      default: ['https://app.brevio.test/oauth/callback']
    },
    tokenExchangeMode: 'simulated',
    stateTtlMs: 600000,
    shutdownTimeoutMs: 1000
  };

  const runtime = createAuthServiceRuntime(config, map);
  try {
    await new Promise<void>((resolve, reject) => {
      runtime.server.listen(0, () => resolve());
      runtime.server.once('error', (err) => reject(err));
    });
  } catch (error) {
    await runtime.close().catch(() => undefined);
    throw error;
  }

  const address = runtime.server.address();
  assert.ok(address && typeof address === 'object' && 'port' in address);
  const port = address.port;
  const baseURL = `http://127.0.0.1:${port}`;
  return {
    runtime,
    baseURL
  };
}

async function startRuntimeOrSkip(t: { skip(message?: string): void }) {
  try {
    return await startRuntime();
  } catch (error) {
    if (error instanceof Error && 'code' in error && error.code === 'EPERM') {
      t.skip('sandbox does not allow binding local ports');
      return null;
    }
    throw error;
  }
}

describe('brevio-auth server', () => {
  it('serves provider map and deep health checks', async (t) => {
    const started = await startRuntimeOrSkip(t);
    if (!started) {
      return;
    }
    const { runtime, baseURL } = started;
    try {
      const providersRes = await fetch(`${baseURL}/api/v1/providers`);
      assert.equal(providersRes.status, 401);
      const adminHeaders = {
        authorization: `Bearer ${serviceToken('brevio-auth', 'admin_access')}`
      };
      const authorizedProvidersRes = await fetch(`${baseURL}/api/v1/providers`, { headers: adminHeaders });
      assert.equal(authorizedProvidersRes.status, 200);
      const providers = (await authorizedProvidersRes.json()) as {
        stats: { oauth_services: number; api_key_services: number; no_auth_services: number };
      };
      assert.equal(providers.stats.oauth_services, 15);
      assert.equal(providers.stats.api_key_services, 18);
      assert.equal(providers.stats.no_auth_services, 6);

      const deepRes = await fetch(`${baseURL}/health/deep`, { headers: adminHeaders });
      assert.equal(deepRes.status, 200);
      const deep = (await deepRes.json()) as {
        checks: { oauth_provider_count: number; api_key_provider_count: number; no_auth_provider_count: number };
      };
      assert.equal(deep.checks.oauth_provider_count, 15);
      assert.equal(deep.checks.api_key_provider_count, 18);
      assert.equal(deep.checks.no_auth_provider_count, 6);
    } finally {
      await runtime.close();
    }
  });

  it('runs pkce authorize and exchange flow', async (t) => {
    const started = await startRuntimeOrSkip(t);
    if (!started) {
      return;
    }
    const { runtime, baseURL } = started;
    try {
      const authorizeRes = await fetch(`${baseURL}/api/v1/oauth/google/authorize`, {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          authorization: `Bearer ${serviceToken('brevio-auth')}`,
          'x-brevio-caller-context': callerContext('u-test-1')
        },
        body: JSON.stringify({ redirect_uri: 'https://app.brevio.test/oauth/callback' })
      });
      assert.equal(authorizeRes.status, 200);
      const authorize = (await authorizeRes.json()) as {
        state: string;
        authorize_url: string;
      };
      assert.match(authorize.state, /^[A-Za-z0-9_-]+$/);
      assert.match(authorize.authorize_url, /code_challenge_method=S256/);

      const exchangeRes = await fetch(`${baseURL}/api/v1/oauth/google/exchange`, {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          authorization: `Bearer ${serviceToken('brevio-auth')}`
        },
        body: JSON.stringify({ state: authorize.state, code: 'abc123' })
      });
      assert.equal(exchangeRes.status, 200);
      const exchange = (await exchangeRes.json()) as {
        status: string;
        token: { access_token: string; refresh_token: string };
      };
      assert.equal(exchange.status, 'success');
      assert.match(exchange.token.access_token, /^access_/);
      assert.match(exchange.token.refresh_token, /^refresh_/);
    } finally {
      await runtime.close();
    }
  });

  it('rejects invalid callback state', async (t) => {
    const started = await startRuntimeOrSkip(t);
    if (!started) {
      return;
    }
    const { runtime, baseURL } = started;
    try {
      const callbackRes = await fetch(`${baseURL}/callback/google?state=invalid&code=test`);
      assert.equal(callbackRes.status, 409);
      const callback = (await callbackRes.json()) as { error: string };
      assert.equal(callback.error, 'invalid_or_expired_state');
    } finally {
      await runtime.close();
    }
  });

  it('disables simulated exchange outside development-style mode', async (t) => {
    const map = loadAuthServiceMap(mapPath);
    const runtime = createAuthServiceRuntime(
      {
        serviceName: 'brevio-auth',
        serviceVersion: 'test',
        environment: 'production',
        port: 0,
        mapPath,
        internalAuthSecret,
        internalAuthIssuer: 'https://auth.brevio.internal',
        serviceAudience: 'brevio-auth',
        callerContextSecret,
        logSalt: 'auth-test-salt',
        stateEncryptionSecret: 'auth-state-secret',
        completionRedirectAllowlist: {
          default: ['https://app.brevio.test/oauth/callback']
        },
        tokenExchangeMode: 'disabled',
        stateTtlMs: 600000,
        shutdownTimeoutMs: 1000
      },
      map
    );

    try {
      await new Promise<void>((resolve, reject) => {
        runtime.server.listen(0, () => resolve());
        runtime.server.once('error', (err) => reject(err));
      });
    } catch (error) {
      await runtime.close().catch(() => undefined);
      if (error instanceof Error && 'code' in error && error.code === 'EPERM') {
        t.skip('sandbox does not allow binding local ports');
        return;
      }
      throw error;
    }

    try {
      const address = runtime.server.address();
      assert.ok(address && typeof address === 'object' && 'port' in address);
      const baseURL = `http://127.0.0.1:${address.port}`;

      const deepRes = await fetch(`${baseURL}/health/deep`, {
        headers: {
          authorization: `Bearer ${serviceToken('brevio-auth', 'admin_access')}`
        }
      });
      assert.equal(deepRes.status, 200);
      const deep = (await deepRes.json()) as { checks: { token_exchange_mode: string } };
      assert.equal(deep.checks.token_exchange_mode, 'disabled');

      const authorizeRes = await fetch(`${baseURL}/api/v1/oauth/google/authorize`, {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          authorization: `Bearer ${serviceToken('brevio-auth')}`,
          'x-brevio-caller-context': callerContext('u-prod-1')
        },
        body: JSON.stringify({ redirect_uri: 'https://app.brevio.test/oauth/callback' })
      });
      assert.equal(authorizeRes.status, 200);
      const authorize = (await authorizeRes.json()) as { state: string };

      const exchangeRes = await fetch(`${baseURL}/api/v1/oauth/google/exchange`, {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          authorization: `Bearer ${serviceToken('brevio-auth')}`
        },
        body: JSON.stringify({ state: authorize.state, code: 'abc123' })
      });
      assert.equal(exchangeRes.status, 503);
      const exchange = (await exchangeRes.json()) as { error: string };
      assert.equal(exchange.error, 'token_exchange_not_configured');
    } finally {
      await runtime.close();
    }
  });
});
