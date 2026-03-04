import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  talking_points_count: number;
  summary: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'coach-success.json'), 'utf8')) as Fixture;
}

describe('relationship-skills integration', () => {
  it('returns deterministic communication coaching output', async () => {
    const result = await adapter.execute(
      {
        action: 'coach_message',
        context: 'I need to discuss timeline concerns without escalating tension.',
        goal: 'Align on a realistic project timeline'
      },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.talking_points?.length, expected.talking_points_count);
    assert.equal(result.data?.summary, expected.summary);
  });
});
