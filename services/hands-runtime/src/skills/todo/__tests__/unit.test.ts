import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('todo unit', () => {
  it('lists items', async () => {
    const output = await runClient({ action: 'list' });
    assert.equal(output.provider, 'todo');
    assert.ok((output.items?.length ?? 0) > 0);
  });

  it('adds item deterministically', async () => {
    const output = await runClient({ action: 'add', content: 'Ship board memo' });
    assert.equal(output.action, 'add');
    assert.ok(output.item_id?.startsWith('todo_'));
  });
});
