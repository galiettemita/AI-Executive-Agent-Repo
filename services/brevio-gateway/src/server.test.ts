import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { createGatewayRuntime } from './index.js';

async function startRuntime() {
  const runtime = createGatewayRuntime({
    serviceName: 'brevio-gateway',
    version: 'test',
    environment: 'test',
    port: 0,
    shutdownTimeoutMs: 1000,
    internalAuthSecret: 'internal-secret',
    internalAuthIssuer: 'https://auth.brevio.internal',
    serviceAudience: 'brevio-gateway',
    temporalWorkerAudience: 'brevio-temporal-worker',
    callerContextSecret: 'caller-secret',
    logSalt: 'gateway-test-salt',
    whatsappWebhookSecret: 'secret',
    whatsappVerifyToken: 'verify',
    imessageAPIKey: 'key',
    temporalWebhookAPIKey: 'temporal',
    temporalWorkerBaseUrl: undefined,
    temporalWorkerTimeoutMs: 1000,
    idempotencyTtlMs: 60_000,
    sessionIdleMs: 60_000,
    rateLimitWindowMs: 60 * 60 * 1000,
    rateLimitMinuteWindowMs: 60 * 1000,
    rateLimitPerMinute: 30,
    rateLimitFreePerHour: 100,
    rateLimitProPerHour: 500
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

describe('brevio-gateway runtime', () => {
  it('returns run and thread ownership metadata on accepted webhook ingress', async (t) => {
    const started = await startRuntimeOrSkip(t);
    if (!started) {
      return;
    }
    const { runtime, baseUrl } = started;
    try {
      const response = await fetch(`${baseUrl}/webhooks/imessage`, {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          'x-api-key': 'key'
        },
        body: JSON.stringify({
          sender_id: '+15551234567',
          message_id: 'msg-123',
          text: 'play music'
        })
      });

      assert.equal(response.status, 202);
      const payload = (await response.json()) as {
        status: string;
        run_id: string;
        thread_id: string;
        message_id: string;
        session_id: string;
      };

      assert.equal(payload.status, 'accepted');
      assert.equal(payload.run_id, payload.message_id);
      assert.equal(payload.thread_id, payload.session_id);
    } finally {
      await runtime.close();
    }
  });
});
