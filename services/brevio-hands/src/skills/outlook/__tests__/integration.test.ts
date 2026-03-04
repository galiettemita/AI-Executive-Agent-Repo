import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  action: string;
  mails_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'inbox-success.json'), 'utf8')) as Fixture;
}

describe('outlook integration', () => {
  it('returns deterministic inbox payload', async () => {
    const result = await adapter.execute({ action: 'inbox_list' }, {} as never);

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.mails?.length, expected.mails_count);
  });
});
