import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('trello unit', () => {
  it('lists cards', async () => {
    const output = await runClient({ action: 'card_list', board_id: 'exec_board' });
    assert.equal(output.provider, 'trello');
    assert.ok((output.cards?.length ?? 0) > 0);
  });

  it('creates card', async () => {
    const output = await runClient({
      action: 'card_create',
      board_id: 'exec_board',
      list_id: 'todo',
      name: 'Prepare metrics deck'
    });

    assert.equal(output.action, 'card_create');
    assert.ok(output.card_id?.startsWith('trl_'));
  });
});
