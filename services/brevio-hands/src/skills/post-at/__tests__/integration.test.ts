import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  checkpoints_count: number;
  latest_status: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'track-success.json'), 'utf8')) as Fixture;

describe('post-at integration', () => {
  it('returns deterministic Austrian Post tracking payload', async () => {
    const result = await adapter.execute(
      { action: 'track_parcel', tracking_number: 'AT123456789' },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.checkpoints?.length, expected.checkpoints_count);
    assert.equal(result.data?.latest_status, expected.latest_status);
  });
});
