import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { tmpdir } from 'node:os';
import { describe, it } from 'node:test';

import { ExecutionStore } from './execution-store.js';

function testStorePath(name: string): string {
  return path.join(mkdtempSync(path.join(tmpdir(), 'brevio-edge-relay-')), `${name}.json`);
}

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

    const sent = store.markSent('req-1', {
      nowMs: 200,
      sessionKey: 'user-1:device-1',
      dispatchReceiptId: 'dispatch-1',
      dispatchLeaseExpiresAt: 230,
      resultDeadlineAt: 500
    });
    assert.equal(sent?.status, 'SENT');

    const acked = store.markAcknowledged('req-1', 'user-1:device-1', 'dispatch-1', 210, 550);
    assert.equal(acked?.status, 'ACKED');

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
        latencyMs: 25,
        sessionKey: 'user-1:device-1',
        dispatchReceiptId: 'dispatch-1',
        resultReceiptId: 'result-1'
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
      'SENT',
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
    assert.equal(store.get('req-2')?.status, 'SENT');
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
      'SENT',
      100
    );

    store.applyResult(
      {
        requestId: 'req-3',
        skillId: 'voice-wake-say',
        status: 'SUCCESS',
        latencyMs: 2,
        resultReceiptId: 'result-1'
      },
      200
    );

    const duplicate = store.applyResult(
      {
        requestId: 'req-3',
        skillId: 'voice-wake-say',
        status: 'SUCCESS',
        latencyMs: 2,
        resultReceiptId: 'result-2'
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
      'SENT',
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
        latencyMs: 2,
        resultReceiptId: 'result-4'
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

  it('fails fast when the persisted snapshot is corrupt', () => {
    const storePath = testStorePath('corrupt');
    writeFileSync(storePath, JSON.stringify({ version: 1, records: [{ requestId: '' }] }), 'utf8');

    assert.throws(() => new ExecutionStore(storePath), /execution state snapshot is corrupt/);
  });

  it('rejects results from the wrong session or receipt', () => {
    const store = new ExecutionStore();
    store.create(
      {
        requestId: 'req-5',
        userId: 'user-1',
        deviceId: 'device-1',
        skillId: 'voice-wake-say'
      },
      'QUEUED',
      100
    );
    store.markSent('req-5', {
      nowMs: 120,
      sessionKey: 'user-1:device-1',
      dispatchReceiptId: 'dispatch-5',
      dispatchLeaseExpiresAt: 180,
      resultDeadlineAt: 500
    });

    const mismatch = store.applyResult(
      {
        requestId: 'req-5',
        skillId: 'voice-wake-say',
        status: 'FAILED',
        latencyMs: 5,
        sessionKey: 'other:device',
        dispatchReceiptId: 'dispatch-5',
        resultReceiptId: 'result-5'
      },
      200
    );

    assert.equal(mismatch.outcome, 'provenance_mismatch');
    assert.equal(store.get('req-5')?.status, 'SENT');
  });
});
