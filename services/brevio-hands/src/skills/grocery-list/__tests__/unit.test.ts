import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('grocery-list adapter', () => {
  it('requires clear confirmation', async () => {
    const result = await adapter.execute({ action: 'clear_list' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /GROCERY_LIST_CLEAR_CONFIRMATION_REQUIRED/);
  });

  it('adds grocery items', async () => {
    const result = await adapter.execute(
      {
        action: 'add_items',
        items: [
          { name: 'Avocado', quantity: 2 },
          { name: 'Oat milk', quantity: 1 }
        ]
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'grocery-list');
    assert.ok((result.data?.total_items ?? 0) >= 2);
  });
});
