import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  action: string;
  results_count: number;
  latency_profile_ms: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'search-subject-success.json'), 'utf8')) as Fixture;
}

describe('apple-mail-search integration', () => {
  it('returns deterministic indexed search payload', async () => {
    const result = await adapter.execute(
      { action: 'search_subject', query: 'board', limit: 10 },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.results?.length, expected.results_count);
    assert.equal(result.data?.latency_profile_ms, expected.latency_profile_ms);
  });
});
