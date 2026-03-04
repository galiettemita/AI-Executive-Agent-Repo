import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  improvements_count: number;
  next_steps_count: number;
  summary: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'log-lesson-success.json'), 'utf8')) as Fixture;
}

describe('self-improvement integration', () => {
  it('returns deterministic lesson capture output', async () => {
    const result = await adapter.execute(
      {
        action: 'log_lesson',
        lesson: 'Blocking deep work before meetings increased output quality.',
        category: 'productivity'
      },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.improvements?.length, expected.improvements_count);
    assert.equal(result.data?.next_steps?.length, expected.next_steps_count);
    assert.equal(result.data?.summary, expected.summary);
  });
});
