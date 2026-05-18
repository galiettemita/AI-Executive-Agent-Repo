import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  mode: string;
  results_count: number;
  first_video_id: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'search-success.json'), 'utf8')) as Fixture;

describe('youtube-api integration', () => {
  it('returns deterministic YouTube search payload', async () => {
    const result = await adapter.execute({ mode: 'search', query: 'AI' }, {} as never);
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.mode, expected.mode);
    assert.equal(result.data?.results?.length, expected.results_count);
    assert.equal(result.data?.results?.[0]?.video_id, expected.first_video_id);
  });
});
