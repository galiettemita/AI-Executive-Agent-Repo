import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { applyExecutionRefs, parseExecutionRefs } from './execution-refs.js';

describe('execution refs helpers', () => {
  it('parses valid runtime refs from a request payload', () => {
    const refs = parseExecutionRefs({
      request_id: 'req-123',
      run_id: 'run-123',
      task_id: 'task-123',
      step_id: 'step-123',
      attempt: 2
    });

    assert.deepEqual(refs, {
      request_id: 'req-123',
      run_id: 'run-123',
      task_id: 'task-123',
      step_id: 'step-123',
      attempt: 2
    });
  });

  it('applies only defined runtime refs to a result payload', () => {
    const payload = applyExecutionRefs(
      {
        skill_id: 'spotify-web-api',
        status: 'SUCCESS'
      },
      {
        request_id: 'req-456',
        step_id: 'step-456'
      }
    );

    assert.deepEqual(payload, {
      request_id: 'req-456',
      step_id: 'step-456',
      skill_id: 'spotify-web-api',
      status: 'SUCCESS'
    });
  });
});
