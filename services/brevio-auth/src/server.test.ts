import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { loadAuthServiceMap } from './config.js';
import { createAuthServiceRuntime } from './server.js';
import type { EnvConfig } from './types.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, '..', '..', '..');
const mapPath = path.join(repoRoot, 'config', 'auth-service-map.yaml');

async function startRuntime() {
  const map = loadAuthServiceMap(mapPath);
  const config: EnvConfig = {
    serviceName: 'brevio-auth',
    serviceVersion: 'test',
    environment: 'test',
    port: 0,
    mapPath,
    stateTtlMs: 600000,
    shutdownTimeoutMs: 1000
  };

  const runtime = createAuthServiceRuntime(config, map);
  await new Promise<void>((resolve, reject) => {
    runtime.server.listen(0, () => resolve());
    runtime.server.once('error', (err) => reject(err));
  });

  const address = runtime.server.address();
  assert.ok(address && typeof address === 'object' && 'port' in address);
  const port = address.port;
  const baseURL = `http://127.0.0.1:${port}`;
  return {
    runtime,
    baseURL
  };
}

describe('brevio-auth server', () => {
  it('serves provider map and deep health checks', async () => {
    const { runtime, baseURL } = await startRuntime();
    try {
      const providersRes = await fetch(`${baseURL}/api/v1/providers`);
      assert.equal(providersRes.status, 200);
      const providers = (await providersRes.json()) as {
        stats: { oauth_services: number; api_key_services: number; no_auth_services: number };
      };
      assert.equal(providers.stats.oauth_services, 15);
      assert.equal(providers.stats.api_key_services, 18);
      assert.equal(providers.stats.no_auth_services, 6);

      const deepRes = await fetch(`${baseURL}/health/deep`);
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

  it('runs pkce authorize and exchange flow', async () => {
    const { runtime, baseURL } = await startRuntime();
    try {
      const authorizeRes = await fetch(`${baseURL}/api/v1/oauth/google/authorize`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ user_id: 'u-test-1' })
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
        headers: { 'content-type': 'application/json' },
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

  it('rejects invalid callback state', async () => {
    const { runtime, baseURL } = await startRuntime();
    try {
      const callbackRes = await fetch(`${baseURL}/callback/google?state=invalid&code=test`);
      assert.equal(callbackRes.status, 409);
      const callback = (await callbackRes.json()) as { error: string };
      assert.equal(callback.error, 'invalid_or_expired_state');
    } finally {
      await runtime.close();
    }
  });
});
