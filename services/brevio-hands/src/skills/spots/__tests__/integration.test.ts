import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  grid_density: string;
  results_count: number;
  top_name: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'search-success.json'), 'utf8')) as Fixture;

describe('spots integration', () => {
  it('returns deterministic exhaustive places search payload', async () => {
    const result = await adapter.execute(
      { query: 'coffee', area: 'downtown', grid_density: 'high', max_results: 5 },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.grid_density, expected.grid_density);
    assert.equal(result.data?.results?.length, expected.results_count);
    assert.equal(result.data?.results?.[0]?.name, expected.top_name);
  });
});
