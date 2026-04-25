import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { tmpdir } from 'node:os';
import { describe, it } from 'node:test';

import { SchedulerStore } from './scheduler-store.js';

function testStatePath(name: string): string {
  return path.join(mkdtempSync(path.join(tmpdir(), 'brevio-scheduler-')), `${name}.json`);
}

describe('SchedulerStore', () => {
  it('persists jobs and triggers when a snapshot path is configured', () => {
    const statePath = testStatePath('state');
    const store = new SchedulerStore(statePath);

    store.saveJob({
      id: 'job-1',
      user_id: 'user-1',
      skill_id: 'todoist',
      schedule: '0 9 * * *',
      timezone: 'UTC',
      status: 'active',
      payload: { list: 'today' },
      next_run_at: '2026-04-09T09:00:00.000Z',
      created_at: '2026-04-09T08:00:00.000Z',
      updated_at: '2026-04-09T08:00:00.000Z'
    });
    store.appendTrigger(
      {
        id: 'trigger-1',
        user_id: 'user-1',
        skill_id: 'todoist',
        payload: { source: 'manual' },
        status: 'queued',
        created_at: '2026-04-09T08:30:00.000Z'
      },
      500
    );

    const reloaded = new SchedulerStore(statePath);
    assert.equal(reloaded.listJobs().length, 1);
    assert.equal(reloaded.getJob('job-1')?.skill_id, 'todoist');
    assert.equal(reloaded.listTriggers().length, 1);
    assert.equal(reloaded.stats().queuedTriggers, 1);
  });

  it('fails fast when the persisted scheduler snapshot is corrupt', () => {
    const statePath = testStatePath('corrupt');
    writeFileSync(statePath, JSON.stringify({ version: 1, jobs: [['job-1', { id: '' }]], triggers: [] }), 'utf8');

    assert.throws(() => new SchedulerStore(statePath), /scheduler state snapshot is corrupt/);
  });
});
