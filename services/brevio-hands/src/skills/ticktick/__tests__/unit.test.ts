import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('ticktick adapter', () => {
  it('requires content when adding task', async () => {
    const result = await adapter.execute({ action: 'add_task' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /TICKTICK_TASK_CONTENT_REQUIRED/);
  });

  it('returns deterministic task listing', async () => {
    const result = await adapter.execute({ action: 'list_tasks' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'ticktick');
    assert.equal(typeof result.data?.total_tasks, 'number');
  });
});
