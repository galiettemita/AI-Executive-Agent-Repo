import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  flights_count: number;
  callsign: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'lookup-success.json'), 'utf8')) as Fixture;

describe('flight-tracker integration', () => {
  it('returns deterministic OpenSky lookup payload', async () => {
    const result = await adapter.execute({ callsign: 'AAL100' }, {} as never);
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.flights?.length, expected.flights_count);
    assert.equal(result.data?.flights?.[0]?.callsign, expected.callsign);
    assert.ok(typeof result.data?.queried_at_utc === 'string');
  });
});
