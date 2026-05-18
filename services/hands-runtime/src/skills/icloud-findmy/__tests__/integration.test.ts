import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  devices_count: number;
  top_name: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'list-success.json'), 'utf8')) as Fixture;

describe('icloud-findmy integration', () => {
  it('returns deterministic Find My device payload', async () => {
    const result = await adapter.execute({}, {} as never);
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.devices?.length, expected.devices_count);
    assert.equal(result.data?.devices?.[0]?.name, expected.top_name);
  });
});
