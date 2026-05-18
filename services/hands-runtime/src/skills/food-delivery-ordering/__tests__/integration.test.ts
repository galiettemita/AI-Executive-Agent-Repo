import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(name: string): unknown {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', name), 'utf8')) as unknown;
}

describe('food-delivery-ordering integration', () => {
  it('returns fixture-backed cart build output', async () => {
    const result = await adapter.execute(
      {
        action: 'build_cart',
        restaurant_id: 'fd_rest_001',
        items: [
          { item_id: 'item_001', quantity: 2 },
          { item_id: 'item_002', quantity: 1 }
        ]
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.deepEqual(result.data, readFixture('build-cart-success.json'));
  });
});
