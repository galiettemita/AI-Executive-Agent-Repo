import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  results_count: number;
  first_url: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'search-success.json'), 'utf8')) as Fixture;

describe('tavily integration', () => {
  it('returns deterministic tavily search results', async () => {
    const result = await adapter.execute(
      {
        query: 'executive systems',
        include_domains: ['example.org'],
        max_results: 3
      },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.results?.length, expected.results_count);
    assert.equal(result.data?.results?.[0]?.url, expected.first_url);
  });
});
