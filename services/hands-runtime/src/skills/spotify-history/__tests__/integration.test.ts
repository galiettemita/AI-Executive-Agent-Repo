import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  top_tracks_count: number;
  total_listening_minutes: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'summary-success.json'), 'utf8')) as Fixture;

describe('spotify-history integration', () => {
  it('returns deterministic listening history payload', async () => {
    const result = await adapter.execute(
      { action: 'listening_summary', window: '4w', limit: 2 },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.top_tracks?.length, expected.top_tracks_count);
    assert.equal(result.data?.total_listening_minutes, expected.total_listening_minutes);
  });
});
