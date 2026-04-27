import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  signTestCallerContext,
  signTestGatewayServiceToken,
  testAccessTokenIssuers,
  testCallerContextIssuers
} from '../../../packages/shared/src/security-test-fixtures.js';
import { createSchedulerRuntime } from './index.js';

async function startRuntime() {
  const runtime = createSchedulerRuntime({
    serviceName: 'brevio-scheduler',
    version: 'test',
    environment: 'test',
    port: 0,
    shutdownTimeoutMs: 1000,
    maxBodyBytes: 128 * 1024,
    maxJobs: 100,
    stateFilePath: undefined,
    accessTokenIssuers: testAccessTokenIssuers(),
    serviceAudience: 'brevio-scheduler',
    callerContextIssuers: testCallerContextIssuers(),
    logSalt: 'scheduler-test-salt'
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

function serviceToken(): string {
  return signTestGatewayServiceToken({
    sub: 'gateway-service',
    aud: 'brevio-scheduler',
    scopes: ['scheduler:write']
  });
}

function callerContext(userId: string, accessToken: string): string {
  return signTestCallerContext({
    aud: 'brevio-scheduler',
    sub: 'gateway-service',
    user_id: userId,
    actor_service: 'brevio-gateway',
    auth_strength: 'service_token',
    provenance: 'gateway.webhook',
    accessToken
  });
}

describe('brevio-scheduler auth', () => {
  it('rejects public deep health access', async (t) => {
    const started = await startRuntimeOrSkip(t);
    if (!started) {
      return;
    }
    const { runtime, baseUrl } = started;
    try {
      const response = await fetch(`${baseUrl}/health/deep`);
      assert.equal(response.status, 401);
    } finally {
      await runtime.close();
    }
  });

  it('binds created jobs to verified caller context rather than payload user_id', async (t) => {
    const started = await startRuntimeOrSkip(t);
    if (!started) {
      return;
    }
    const { runtime, baseUrl } = started;
    try {
      const token = serviceToken();
      const response = await fetch(`${baseUrl}/api/v1/scheduler/jobs`, {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          authorization: `Bearer ${token}`,
          'x-brevio-caller-context': callerContext('user-12345678', token)
        },
        body: JSON.stringify({
          user_id: 'spoofed-user',
          skill_id: 'apple-remind-me',
          schedule: '0 9 * * *'
        })
      });

      assert.equal(response.status, 201);
      const payload = (await response.json()) as {
        job: { user_id: string };
      };
      assert.equal(payload.job.user_id, 'user-12345678');
    } finally {
      await runtime.close();
    }
  });
});
