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

describe('hotel-vacation-booking integration', () => {
  it('returns fixture-backed hotel search results', async () => {
    const result = await adapter.execute(
      {
        action: 'search_hotels',
        city: 'Austin',
        check_in: '2026-04-01',
        check_out: '2026-04-03',
        guests: 2
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.deepEqual(result.data, readFixture('search-hotels-success.json'));
  });
});
