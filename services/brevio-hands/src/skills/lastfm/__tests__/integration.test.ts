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
  top_track: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'top-tracks-success.json'), 'utf8')) as Fixture;

describe('lastfm integration', () => {
  it('returns deterministic top tracks payload', async () => {
    const result = await adapter.execute({ action: 'top_tracks', username: 'exec_user' }, {} as never);
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.tracks?.length, expected.tracks_count);
    assert.equal(result.data?.tracks?.[0]?.name, expected.top_track);
  });
});
