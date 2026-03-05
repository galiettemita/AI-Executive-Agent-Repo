import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  action: string;
  posts_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'timeline-success.json'), 'utf8')) as Fixture;
}

describe('bluesky integration', () => {
  it('returns deterministic timeline payload', async () => {
    const result = await adapter.execute({ action: 'timeline' }, {} as never);

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.posts?.length, expected.posts_count);
  });
});
