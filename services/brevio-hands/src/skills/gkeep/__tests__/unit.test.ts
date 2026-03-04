import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('gkeep unit', () => {
  it('lists notes', async () => {
    const output = await runClient({ action: 'list' });
    assert.equal(output.provider, 'gkeep');
    assert.ok((output.notes?.length ?? 0) > 0);
  });

  it('creates note', async () => {
    const output = await runClient({
      action: 'create',
      title: 'Ops memo',
      content: 'Track decisions and follow-ups.'
    });

    assert.equal(output.action, 'create');
    assert.ok(output.note_id?.startsWith('gkeep_'));
  });
});
