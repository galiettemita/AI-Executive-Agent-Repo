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

describe('bill-pay-p2p integration', () => {
  it('returns fixture-backed payee list', async () => {
    const result = await adapter.execute({ action: 'list_payees' }, {} as never);

    assert.equal(result.status, 'SUCCESS');
    assert.deepEqual(result.data, readFixture('list-payees-success.json'));
  });
});
