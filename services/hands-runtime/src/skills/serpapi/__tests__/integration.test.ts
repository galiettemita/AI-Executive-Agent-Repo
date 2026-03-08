import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  engine: string;
  results_count: number;
  top_title: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'search-success.json'), 'utf8')) as Fixture;

describe('serpapi integration', () => {
  it('returns deterministic engine-specific search results', async () => {
    const result = await adapter.execute(
      { query: 'bistro', engine: 'yelp', max_results: 5 },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.engine, expected.engine);
    assert.equal(result.data?.results?.length, expected.results_count);
    assert.equal(result.data?.results?.[0]?.title, expected.top_title);
  });
});
