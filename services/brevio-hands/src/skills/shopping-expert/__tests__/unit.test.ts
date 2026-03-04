import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('shopping-expert unit', () => {
  it('filters by max price and query', async () => {
    const output = await runClient({
      query: 'running shoes',
      max_price: 100,
      limit: 3
    });

    assert.equal(output.provider, 'mock_catalog');
    assert.ok(output.results.length > 0);
    assert.ok(output.results.every((item) => item.price <= 100));
  });
});
