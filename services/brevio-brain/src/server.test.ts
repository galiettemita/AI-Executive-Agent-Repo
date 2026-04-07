import assert from 'node:assert/strict';
import path from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import { createBrainRuntime } from './index.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, '..', '..', '..');
const configPath = path.join(repoRoot, 'config', 'skill-disambiguation.yaml');

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
    plannerBaseUrl: 'https://api.openai.com/v1'
  });

  await new Promise<void>((resolve, reject) => {
    runtime.server.listen(0, () => resolve());
    runtime.server.once('error', (err) => reject(err));
  });

  const address = runtime.server.address();
  assert.ok(address && typeof address === 'object' && 'port' in address);
  const baseURL = `http://127.0.0.1:${address.port}`;
  return { runtime, baseURL };
}

describe('brevio-brain runtime', () => {
  it('rejects invalid JSON payloads', async () => {
    const { runtime, baseURL } = await startRuntime();
    try {
      const response = await fetch(`${baseURL}/api/v1/brain/classify`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: '{bad json'
      });

      assert.equal(response.status, 400);
      const payload = (await response.json()) as { error: string };
      assert.equal(payload.error, 'invalid_json');
    } finally {
      await runtime.close();
    }
  });

  it('returns dispatch_ready for plans that have not executed yet', async () => {
    const { runtime, baseURL } = await startRuntime();
    try {
      const response = await fetch(`${baseURL}/api/v1/brain/process`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
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
        execution_status: string;
        aggregation?: unknown;
      };
      assert.equal(payload.execution_status, 'dispatch_ready');
      assert.equal(payload.aggregation, undefined);
    } finally {
      await runtime.close();
    }
  });

  it('returns completed only when real skill results are provided', async () => {
    const { runtime, baseURL } = await startRuntime();
    try {
      const response = await fetch(`${baseURL}/api/v1/brain/process`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({
          message_text: 'play music',
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
        execution_status: string;
        aggregation: { completion_ratio: number };
      };
      assert.equal(payload.execution_status, 'completed');
      assert.equal(payload.aggregation.completion_ratio, 1);
    } finally {
      await runtime.close();
    }
  });
});
