import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { ExecutionStore } from './execution-store.js';

describe('ExecutionStore', () => {
  it('tracks lifecycle transitions from queue to dispatch to completion', () => {
    const store = new ExecutionStore();
    store.create(
      {
        requestId: 'req-1',
        userId: 'user-1',
        deviceId: 'device-1',
        skillId: 'voice-wake-say',
        runId: 'run-1',
        taskId: 'task-1',
        stepId: 'step-1',
        attempt: 1
      },
      'QUEUED',
      100
    );

    const dispatched = store.markDispatched('req-1', 200);
    assert.equal(dispatched?.status, 'DISPATCHED');

    const applied = store.applyResult(
      {
        requestId: 'req-1',
        skillId: 'voice-wake-say',
        runId: 'run-1',
        taskId: 'task-1',
        stepId: 'step-1',
        attempt: 1,
        status: 'SUCCESS',
        data: { spoken_text: 'done' },
        latencyMs: 25
      },
      300
    );

    assert.equal(applied.outcome, 'applied');
    assert.equal(applied.record?.status, 'SUCCESS');
    assert.equal(applied.record?.completedAt, 300);
    assert.equal(applied.record?.workflowReport?.status, 'PENDING');
  });

  it('rejects mismatched results without mutating the record', () => {
    const store = new ExecutionStore();
    store.create(
      {
        requestId: 'req-2',
        userId: 'user-1',
        deviceId: 'device-1',
        skillId: 'voice-wake-say',
        stepId: 'step-1'
      },
      'DISPATCHED',
      100
    );

    const mismatch = store.applyResult(
      {
        requestId: 'req-2',
        skillId: 'apple-remind-me',
        stepId: 'step-1',
        status: 'FAILED',
        error: { code: 'NOPE', message: 'nope' },
        latencyMs: 1
      },
      200
    );

    assert.equal(mismatch.outcome, 'skill_mismatch');
    assert.equal(store.get('req-2')?.status, 'DISPATCHED');
  });

  it('treats duplicate terminal results idempotently', () => {
    const store = new ExecutionStore();
    store.create(
      {
        requestId: 'req-3',
        userId: 'user-1',
        deviceId: 'device-1',
        skillId: 'voice-wake-say'
      },
      'DISPATCHED',
      100
    );

    store.applyResult(
      {
        requestId: 'req-3',
        skillId: 'voice-wake-say',
        status: 'SUCCESS',
        latencyMs: 2
      },
      200
    );

    const duplicate = store.applyResult(
      {
        requestId: 'req-3',
        skillId: 'voice-wake-say',
        status: 'SUCCESS',
        latencyMs: 2
      },
      300
    );

    assert.equal(duplicate.outcome, 'duplicate');
    assert.equal(store.get('req-3')?.completedAt, 200);
  });

  it('tracks workflow report retries until the worker acknowledges persistence', () => {
    const store = new ExecutionStore();
    store.create(
      {
        requestId: 'req-4',
        userId: 'user-1',
        deviceId: 'device-1',
        skillId: 'voice-wake-say',
        runId: 'run-1',
        stepId: 'step-1'
      },
      'DISPATCHED',
      100
    );

    store.applyResult(
      {
        requestId: 'req-4',
        skillId: 'voice-wake-say',
        runId: 'run-1',
        stepId: 'step-1',
        status: 'FAILED',
        error: { code: 'boom', message: 'boom' },
        latencyMs: 2
      },
      200
    );

    const retrying = store.markWorkflowReportOutcome('req-4', false, 250, 'worker unavailable', 400);
    assert.equal(retrying?.workflowReport?.status, 'RETRYING');
    assert.equal(retrying?.workflowReport?.attempts, 1);
    assert.equal(store.pendingWorkflowReports(399).length, 0);
    assert.equal(store.pendingWorkflowReports(400).length, 1);

    const delegated = store.markWorkflowReportOutcome('req-4', true, 450, undefined);
    assert.equal(delegated?.workflowReport?.status, 'DELEGATED');
  });
});
