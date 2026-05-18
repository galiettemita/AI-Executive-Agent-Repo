import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  action: string;
  files_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'drive-search-success.json'), 'utf8')) as Fixture;
}

describe('google-workspace integration', () => {
  it('returns deterministic drive search payload', async () => {
    const result = await adapter.execute(
      { action: 'drive_search', query: 'q2' },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.files?.length, expected.files_count);
  });
});
