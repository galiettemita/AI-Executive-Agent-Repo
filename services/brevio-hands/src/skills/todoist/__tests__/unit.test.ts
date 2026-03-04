import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('todoist unit', () => {
  it('creates task with deterministic id', async () => {
    const output = await runClient({
      action: 'create',
      project_id: 'work',
      task: {
        content: 'Finalize board memo',
        due_string: 'tomorrow 9am',
        priority: 4
      }
    });

    assert.equal(output.provider, 'todoist_mock');
    assert.equal(output.action, 'create');
    assert.ok(output.task_id?.startsWith('task_'));
  });

  it('lists tasks', async () => {
    const output = await runClient({ action: 'list', project_id: 'inbox' });
    assert.equal(output.action, 'list');
    assert.ok((output.tasks?.length ?? 0) > 0);
  });
});
