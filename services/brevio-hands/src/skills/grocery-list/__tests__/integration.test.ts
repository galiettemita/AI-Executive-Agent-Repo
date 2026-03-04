import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  total_items: number;
  items_count: number;
  new_item_name: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'add-items-success.json'), 'utf8')) as Fixture;

describe('grocery-list integration', () => {
  it('returns deterministic grocery list mutation payload', async () => {
    const result = await adapter.execute(
      {
        action: 'add_items',
        list_id: 'weekly',
        items: [{ name: 'apples', quantity: 2 }]
      },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.total_items, expected.total_items);
    assert.equal(result.data?.items?.length, expected.items_count);
    assert.equal(
      result.data?.items?.some((item: { name: string }) => item.name === expected.new_item_name),
      true
    );
  });
});
