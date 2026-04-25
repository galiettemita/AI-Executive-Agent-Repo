import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { signAccessToken } from '../../../packages/shared/src/security.js';
import { createProfileRuntime } from './index.js';

async function startRuntime() {
  const runtime = createProfileRuntime({
    serviceName: 'brevio-profile',
    version: 'test',
    environment: 'test',
    port: 0,
    shutdownTimeoutMs: 1000,
    profilesRootDir: '.tmp-profile-tests',
    maxKnowledgeBytes: 128 * 1024,
    internalAuthSecret: 'internal-secret',
    internalAuthIssuer: 'https://auth.brevio.internal',
    serviceAudience: 'brevio-profile',
    callerContextSecret: 'caller-secret',
    logSalt: 'profile-test-salt'
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

function userToken(userId: string): string {
  const nowSeconds = Math.floor(Date.now() / 1000);
  return signAccessToken('internal-secret', {
    version: 2,
    sub: userId,
    iss: 'https://auth.brevio.internal',
    aud: 'brevio-profile',
    iat: nowSeconds,
    exp: nowSeconds + 60,
    token_use: 'user_access'
  });
}

describe('brevio-profile auth', () => {
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

  it('forbids user tokens from reading another user profile', async (t) => {
    const started = await startRuntimeOrSkip(t);
    if (!started) {
      return;
    }
    const { runtime, baseUrl } = started;
    try {
      const response = await fetch(`${baseUrl}/api/v1/profile/user-87654321`, {
        headers: {
          authorization: `Bearer ${userToken('user-12345678')}`
        }
      });
      assert.equal(response.status, 403);
    } finally {
      await runtime.close();
    }
  });
});
