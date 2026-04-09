import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { reportExecutionLifecycle } from './workflow-runtime.js';

describe('edge relay workflow runtime reporting', () => {
  it('reports accepted terminal results to the worker', async () => {
    const result = await reportExecutionLifecycle(
      {
        requestId: 'req-1',
        userId: 'user-1',
        deviceId: 'device-1',
        skillId: 'voice-wake-say',
        runId: 'run-1',
        taskId: 't1',
        stepId: 'step_t1',
        attempt: 1,
        status: 'SUCCESS',
        createdAt: 1,
        updatedAt: 2,
        completedAt: 2,
        result: {
          status: 'SUCCESS',
          data: { spoken_text: 'done' },
          latencyMs: 20
        }
      },
      {
        temporalWorkerBaseUrl: 'http://runtime.local',
        temporalWorkerTimeoutMs: 1000
      },
      async (url, init) => {
        assert.equal(url, 'http://runtime.local/api/v1/temporal-worker/runs/run-1/planner-steps/step_t1/transition');
        assert.equal(init?.method, 'POST');
        const payload = JSON.parse(String(init?.body)) as Record<string, unknown>;
        assert.equal(payload.status, 'COMPLETED');
        return new Response(JSON.stringify({ step_id: 'internal-step-1' }), { status: 200 });
      }
    );

    assert.deepEqual(result, { delegated: true });
  });

  it('does nothing for non-terminal or uncorrelated records', async () => {
    const result = await reportExecutionLifecycle(
      {
        requestId: 'req-2',
        userId: 'user-1',
        deviceId: 'device-1',
        skillId: 'voice-wake-say',
        status: 'DISPATCHED',
        createdAt: 1,
        updatedAt: 2
      },
      {
        temporalWorkerBaseUrl: 'http://runtime.local',
        temporalWorkerTimeoutMs: 1000
      }
    );

    assert.deepEqual(result, { delegated: false });
  });
});
