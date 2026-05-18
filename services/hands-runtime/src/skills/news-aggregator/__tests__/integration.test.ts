import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  items_count: number;
  top_source: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'aggregate-success.json'), 'utf8')) as Fixture;

describe('news-aggregator integration', () => {
  it('returns deterministic aggregated feed payload', async () => {
    const result = await adapter.execute({ topic: 'ops', max_items: 10 }, {} as never);
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.items?.length, expected.items_count);
    assert.equal(result.data?.items?.[0]?.source, expected.top_source);
  });
});
