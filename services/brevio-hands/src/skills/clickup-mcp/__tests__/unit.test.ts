import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('clickup-mcp unit', () => {
  it('lists tasks', async () => {
    const output = await runClient({ action: 'task_list' });
    assert.equal(output.provider, 'clickup-mcp');
    assert.ok((output.tasks?.length ?? 0) > 0);
  });

  it('creates doc', async () => {
    const output = await runClient({ action: 'doc_create', title: 'Ops Notes' });
    assert.equal(output.action, 'doc_create');
    assert.ok(output.doc_id?.startsWith('doc_'));
  });
});
