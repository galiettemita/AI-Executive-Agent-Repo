import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(name: string): unknown {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', name), 'utf8')) as unknown;
}

describe('kids-family-management integration', () => {
  it('returns fixture-backed family schedule', async () => {
    const result = await adapter.execute({ action: 'family_schedule' }, {} as never);

    assert.equal(result.status, 'SUCCESS');
    assert.deepEqual(result.data, readFixture('family-schedule-success.json'));
  });
});
