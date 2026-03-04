import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('doing-tasks adapter', () => {
  it('requires task for route_task action', async () => {
    const result = await adapter.execute({ action: 'route_task' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /DOING_TASKS_TASK_REQUIRED/);
  });

  it('returns deterministic execution plan', async () => {
    const result = await adapter.execute({ action: 'route_task', task: 'Draft investor update' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'doing-tasks');
  });
});
