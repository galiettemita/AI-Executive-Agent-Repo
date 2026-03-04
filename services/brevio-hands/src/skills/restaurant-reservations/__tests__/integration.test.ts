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

describe('restaurant-reservations integration', () => {
  it('returns fixture-backed search results', async () => {
    const result = await adapter.execute(
      { action: 'search', city: 'Austin', date: '2026-03-10', party_size: 2 },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.deepEqual(result.data, readFixture('search-success.json'));
  });
});
