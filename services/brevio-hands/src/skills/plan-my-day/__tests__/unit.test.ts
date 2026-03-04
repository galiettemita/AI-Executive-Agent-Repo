import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('plan-my-day adapter', () => {
  it('requires tasks for build_plan', async () => {
    const result = await adapter.execute(
      {
        action: 'build_plan',
        timezone: 'America/Los_Angeles',
        date: '2026-03-04'
      },
      {} as never
    );
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /PLAN_MY_DAY_TASKS_REQUIRED/);
  });

  it('builds time blocks from tasks', async () => {
    const result = await adapter.execute(
      {
        action: 'build_plan',
        timezone: 'America/Los_Angeles',
        date: '2026-03-04',
        tasks: [
          { title: 'Board prep', duration_minutes: 90, priority: 'high', energy: 'deep' },
          { title: 'Inbox triage', duration_minutes: 30, priority: 'medium', energy: 'admin' }
        ]
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'plan-my-day');
    assert.ok(Array.isArray(result.data?.time_blocks));
  });
});
