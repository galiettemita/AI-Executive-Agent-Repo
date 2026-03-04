import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  carrier: string;
  status: string;
  history_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'track-success.json'), 'utf8')) as Fixture;

describe('parcel-package-tracking integration', () => {
  it('returns deterministic parcel tracking timeline', async () => {
    const result = await adapter.execute({ tracking_number: '1Z999AA10123456784' }, {} as never);
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.carrier, expected.carrier);
    assert.equal(result.data?.status, expected.status);
    assert.equal(result.data?.history?.length, expected.history_count);
  });
});
