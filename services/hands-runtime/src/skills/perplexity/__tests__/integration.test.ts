import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  citations_count: number;
  answer_prefix: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'answer-success.json'), 'utf8')) as Fixture;

describe('perplexity integration', () => {
  it('returns deterministic grounded answer payload', async () => {
    const result = await adapter.execute(
      { query: 'summarize executive assistant tooling', model: 'sonar-medium-online' },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.citations?.length, expected.citations_count);
    assert.equal((result.data?.answer as string).startsWith(expected.answer_prefix), true);
  });
});
