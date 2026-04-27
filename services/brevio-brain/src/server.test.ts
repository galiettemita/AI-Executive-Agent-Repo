import assert from 'node:assert/strict';
import path from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import {
  signTestCallerContext,
  signTestGatewayServiceToken,
  testAccessTokenIssuers,
  testCallerContextIssuers
} from '../../../packages/shared/src/security-test-fixtures.js';
import { createBrainRuntime } from './index.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, '..', '..', '..');
const configPath = path.join(repoRoot, 'config', 'skill-disambiguation.yaml');
function authHeaders(userId = 'u-brain-test') {
  const accessToken = signTestGatewayServiceToken({
    sub: 'brevio-gateway',
    aud: 'brevio-brain',
    scopes: ['brain:test']
  });
  return {
    'content-type': 'application/json',
    authorization: `Bearer ${accessToken}`,
    'x-brevio-caller-context': signTestCallerContext({
      aud: 'brevio-brain',
      sub: userId,
      user_id: userId,
      actor_service: 'brevio-gateway',
      auth_strength: 'service_token',
      provenance: 'test',
      accessToken
    })
  };
}

async function startRuntime() {
  const runtime = createBrainRuntime({
    serviceName: 'brevio-brain',
    version: 'test',
    environment: 'test',
    port: 0,
    shutdownTimeoutMs: 1000,
    disambiguationConfigPath: configPath,
    plannerProvider: 'deterministic',
    plannerModel: 'gpt-5.2',
    plannerFallbackModel: 'gpt-5-mini',
    plannerTimeoutMs: 1000,
    plannerBaseUrl: 'https://api.openai.com/v1',
    temporalWorkerBaseUrl: undefined,
    temporalWorkerTimeoutMs: 1000,
    accessTokenIssuers: testAccessTokenIssuers(),
    serviceAudience: 'brevio-brain',
    callerContextIssuers: testCallerContextIssuers(),
    logSalt: 'brain-test-salt'
  });

  await new Promise<void>((resolve, reject) => {
    runtime.server.listen(0, '127.0.0.1', () => resolve());
    runtime.server.once('error', (err) => reject(err));
  });

  const address = runtime.server.address();
  assert.ok(address && typeof address === 'object' && 'port' in address);
  const baseURL = `http://127.0.0.1:${address.port}`;
  return { runtime, baseURL };
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

describe('brevio-brain runtime', () => {
  it('rejects invalid JSON payloads', async (t) => {
    const started = await startRuntimeOrSkip(t);
    if (!started) {
      return;
    }
    const { runtime, baseURL } = started;
    try {
      const response = await fetch(`${baseURL}/api/v1/brain/classify`, {
        method: 'POST',
        headers: authHeaders(),
        body: '{bad json'
      });

      assert.equal(response.status, 400);
      const payload = (await response.json()) as { error: string };
      assert.equal(payload.error, 'invalid_json');
    } finally {
      await runtime.close();
    }
  });

  it('returns dispatch_ready for plans that have not executed yet', async (t) => {
    const started = await startRuntimeOrSkip(t);
    if (!started) {
      return;
    }
    const { runtime, baseURL } = started;
    try {
      const response = await fetch(`${baseURL}/api/v1/brain/process`, {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({
          message_text: 'play music',
          user_profile: {
            enabled_skills: ['spotify-web-api']
          },
          user_preferences: { music_provider: 'spotify' }
        })
      });

      assert.equal(response.status, 200);
      const payload = (await response.json()) as {
        run_id: string;
        thread_id: string;
        execution_status: string;
        aggregation?: unknown;
      };
      assert.ok(payload.run_id.length > 0);
      assert.equal(payload.thread_id, payload.run_id);
      assert.equal(payload.execution_status, 'dispatch_ready');
      assert.equal(payload.aggregation, undefined);
    } finally {
      await runtime.close();
    }
  });

  it('returns completed only when real skill results are provided', async (t) => {
    const started = await startRuntimeOrSkip(t);
    if (!started) {
      return;
    }
    const { runtime, baseURL } = started;
    try {
      const response = await fetch(`${baseURL}/api/v1/brain/process`, {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({
          message_text: 'play music',
          run_id: 'run-server-123',
          thread_id: 'thread-server-123',
          user_profile: {
            enabled_skills: ['spotify-web-api']
          },
          user_preferences: { music_provider: 'spotify' },
          skill_results: [
            {
              skill_id: 'spotify-web-api',
              status: 'SUCCESS',
              data: {
                summary: 'Playback started.'
              }
            }
          ]
        })
      });

      assert.equal(response.status, 200);
      const payload = (await response.json()) as {
        run_id: string;
        thread_id: string;
        execution_status: string;
        aggregation: { completion_ratio: number };
      };
      assert.equal(payload.run_id, 'run-server-123');
      assert.equal(payload.thread_id, 'thread-server-123');
      assert.equal(payload.execution_status, 'completed');
      assert.equal(payload.aggregation.completion_ratio, 1);
    } finally {
      await runtime.close();
    }
  });
});
