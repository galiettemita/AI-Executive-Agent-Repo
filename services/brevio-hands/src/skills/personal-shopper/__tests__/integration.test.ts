import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  candidate_count: number;
  top_candidate: string;
  recommendation_contains: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'research-success.json'), 'utf8')) as Fixture;
}

describe('personal-shopper integration', () => {
  it('returns deterministic ranked candidates', async () => {
    const result = await adapter.execute(
      { action: 'research_product', query: 'Laptop', budget_cents: 120000 },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.ranked_candidates?.length, expected.candidate_count);
    assert.equal(result.data?.ranked_candidates?.[0]?.name, expected.top_candidate);
    assert.match(result.data?.recommendation ?? '', new RegExp(expected.recommendation_contains));
  });
});
