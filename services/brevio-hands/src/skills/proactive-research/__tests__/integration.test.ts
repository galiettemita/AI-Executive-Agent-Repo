import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  alerts_count: number;
  next_check_at: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'monitor-success.json'), 'utf8')) as Fixture;
}

describe('proactive-research integration', () => {
  it('returns deterministic monitoring payload', async () => {
    const result = await adapter.execute(
      { action: 'monitor_topic', topic: 'Competitor pricing' },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.alerts?.length, expected.alerts_count);
    assert.equal(result.data?.next_check_at, expected.next_check_at);
  });
});
