import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  status: string;
  checkpoints_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'track-success.json'), 'utf8')) as Fixture;

describe('track17 integration', () => {
  it('returns deterministic 17track checkpoints payload', async () => {
    const result = await adapter.execute({ tracking_number: 'LG123456789CN' }, {} as never);
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.status, expected.status);
    assert.equal(result.data?.checkpoints?.length, expected.checkpoints_count);
  });
});
