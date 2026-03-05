import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  action: string;
  channels_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'list-channels-success.json'), 'utf8')) as Fixture;
}

describe('slack integration', () => {
  it('returns deterministic channel-list payload', async () => {
    const result = await adapter.execute({ action: 'list_channels' }, {} as never);

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.channels?.length, expected.channels_count);
  });
});
