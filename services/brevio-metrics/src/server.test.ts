import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { signAccessToken } from '../../../packages/shared/src/security.js';
import { createMetricsRuntime } from './index.js';

async function startRuntime() {
  const runtime = createMetricsRuntime({
    serviceName: 'brevio-metrics',
    version: 'test',
    environment: 'test',
    port: 0,
    shutdownTimeoutMs: 1000,
    maxBodyBytes: 128 * 1024,
    stateFilePath: undefined,
    internalAuthSecret: 'internal-secret',
    internalAuthIssuer: 'https://auth.brevio.internal',
    serviceAudience: 'brevio-metrics',
    logSalt: 'metrics-test-salt'
  });

  await new Promise<void>((resolve, reject) => {
    runtime.server.listen(0, '127.0.0.1', () => resolve());
    runtime.server.once('error', (err) => reject(err));
  });

  const address = runtime.server.address();
  assert.ok(address && typeof address === 'object' && 'port' in address);
  return {
    runtime,
    baseUrl: `http://127.0.0.1:${address.port}`
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

function adminToken(): string {
  const nowSeconds = Math.floor(Date.now() / 1000);
  return signAccessToken('internal-secret', {
    version: 2,
    sub: 'admin-user',
    iss: 'https://auth.brevio.internal',
    aud: 'brevio-metrics',
    iat: nowSeconds,
    exp: nowSeconds + 60,
    token_use: 'admin_access'
  });
}

describe('brevio-metrics auth', () => {
  it('keeps Prometheus metrics public while protecting deep health', async (t) => {
    const started = await startRuntimeOrSkip(t);
    if (!started) {
      return;
    }
    const { runtime, baseUrl } = started;
    try {
      const publicMetrics = await fetch(`${baseUrl}/metrics`);
      assert.equal(publicMetrics.status, 200);

      const deepHealth = await fetch(`${baseUrl}/health/deep`);
      assert.equal(deepHealth.status, 401);

      const authedDeepHealth = await fetch(`${baseUrl}/health/deep`, {
        headers: {
          authorization: `Bearer ${adminToken()}`
        }
      });
      assert.equal(authedDeepHealth.status, 200);
    } finally {
      await runtime.close();
    }
  });
});
