import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { startMessageWorkflow } from './workflow-runtime.js';

describe('gateway workflow runtime', () => {
  it('starts a message workflow when the worker is configured', async () => {
    const result = await startMessageWorkflow(
      {
        messageId: 'msg-1',
        userId: 'user-1',
        channel: 'IMESSAGE',
        channelMessageId: 'channel-msg-1',
        sessionId: 'session-1',
        messageText: 'play music',
        userProfileHash: 'profile-1'
      },
      {
        temporalWorkerBaseUrl: 'http://runtime.local',
        temporalWorkerTimeoutMs: 1000
      },
      async (url, init) => {
        assert.equal(url, 'http://runtime.local/api/v1/temporal-worker/workflows/message-processing');
        assert.equal(init?.method, 'POST');
        const payload = JSON.parse(String(init?.body)) as Record<string, unknown>;
        assert.equal(payload.pause_after_state, 'RECEIVED');
        assert.equal(payload.message_id, 'msg-1');
        return new Response(JSON.stringify({ run_id: 'run-1' }), { status: 202 });
      }
    );

    assert.deepEqual(result, {
      delegated: true,
      runId: 'run-1'
    });
  });

  it('falls back safely when the runtime is unavailable', async () => {
    const result = await startMessageWorkflow(
      {
        messageId: 'msg-2',
        userId: 'user-2',
        channel: 'API',
        channelMessageId: 'channel-msg-2',
        sessionId: 'session-2',
        userProfileHash: 'profile-2'
      },
      {
        temporalWorkerBaseUrl: 'http://runtime.local',
        temporalWorkerTimeoutMs: 1000
      },
      async () => {
        throw new Error('connection refused');
      }
    );

    assert.equal(result.delegated, false);
    assert.equal(result.warning, 'temporal_worker_start_unavailable');
  });
});
