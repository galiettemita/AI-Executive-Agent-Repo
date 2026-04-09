import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { reportExecutionResult } from './workflow-runtime.js';

describe('hands workflow runtime reporting', () => {
  it('reports successful execution results to the worker', async () => {
    const result = await reportExecutionResult(
      {
        request_id: 'req-1',
        run_id: 'run-1',
        task_id: 't1',
        step_id: 'step_t1',
        attempt: 1,
        skill_id: 'apple-mail-search',
        status: 'SUCCESS',
        data: { matches: 3 },
        latency_ms: 25
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

  it('falls back safely when reporting is unavailable', async () => {
    const result = await reportExecutionResult(
      {
        run_id: 'run-2',
        step_id: 'step_t2',
        skill_id: 'todoist',
        status: 'FAILED',
        error: { code: 'NOPE', message: 'nope' },
        latency_ms: 10
      },
      {
        temporalWorkerBaseUrl: 'http://runtime.local',
        temporalWorkerTimeoutMs: 1000
      },
      async () => {
        throw new Error('network down');
      }
    );

    assert.equal(result.delegated, false);
    assert.equal(result.warning, 'temporal_worker_result_report_unavailable');
  });
});
