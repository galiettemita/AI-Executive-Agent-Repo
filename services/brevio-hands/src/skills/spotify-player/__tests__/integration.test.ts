import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  tracks_count: number;
  queue_length: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'search-success.json'), 'utf8')) as Fixture;

describe('spotify-player integration', () => {
  it('returns deterministic terminal search payload', async () => {
    const result = await adapter.execute(
      { action: 'search_tracks', query: 'deep' },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.tracks?.length, expected.tracks_count);
    assert.equal(result.data?.queue_length, expected.queue_length);
  });
});
