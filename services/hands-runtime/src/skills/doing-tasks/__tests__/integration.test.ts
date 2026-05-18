import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  routed_skill: string;
  execution_plan_count: number;
  summary: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'route-success.json'), 'utf8')) as Fixture;
}

describe('doing-tasks integration', () => {
  it('returns deterministic routing metadata', async () => {
    const result = await adapter.execute(
      { action: 'route_task', task: 'Prepare quarterly board memo', skill_hint: 'todoist' },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.routed_skill, expected.routed_skill);
    assert.equal(result.data?.execution_plan?.length, expected.execution_plan_count);
    assert.equal(result.data?.summary, expected.summary);
  });
});
