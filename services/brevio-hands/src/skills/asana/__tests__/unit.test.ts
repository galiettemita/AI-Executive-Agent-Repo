import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('asana unit', () => {
  it('lists tasks by project', async () => {
    const output = await runClient({ action: 'task_list', project_id: 'proj_exec' });
    assert.equal(output.provider, 'asana');
    assert.ok((output.tasks?.length ?? 0) > 0);
  });

  it('creates task', async () => {
    const output = await runClient({
      action: 'task_create',
      project_id: 'proj_exec',
      name: 'Prepare steering committee notes'
    });

    assert.equal(output.action, 'task_create');
    assert.ok(output.task_id?.startsWith('asn_'));
  });
});
