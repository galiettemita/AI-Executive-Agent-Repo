import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  mode: string;
  distance_m: number;
  steps_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'route-success.json'), 'utf8')) as Fixture;

describe('google-maps integration', () => {
  it('returns deterministic route payload', async () => {
    const result = await adapter.execute(
      {
        origin: 'Austin, TX',
        destination: 'Houston, TX',
        mode: 'driving'
      },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.mode, expected.mode);
    assert.equal(result.data?.distance_m, expected.distance_m);
    assert.equal(result.data?.steps?.length, expected.steps_count);
  });
});
