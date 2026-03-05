import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  time_block_count: number;
  first_block: { title: string; start_local: string; end_local: string };
  overflow_count: number;
  strategy_note_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'build-plan-success.json'), 'utf8')) as Fixture;
}

describe('plan-my-day integration', () => {
  it('returns deterministic schedule allocation', async () => {
    const result = await adapter.execute(
      {
        action: 'build_plan',
        timezone: 'America/New_York',
        date: '2026-03-05',
        tasks: [
          { title: 'Deep Strategy Review', duration_minutes: 60, priority: 'high', energy: 'deep' },
          { title: 'Inbox Processing', duration_minutes: 30, priority: 'medium', energy: 'admin' }
        ]
      },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.time_blocks?.length, expected.time_block_count);
    assert.equal(result.data?.time_blocks?.[0]?.title, expected.first_block.title);
    assert.equal(result.data?.time_blocks?.[0]?.start_local, expected.first_block.start_local);
    assert.equal(result.data?.time_blocks?.[0]?.end_local, expected.first_block.end_local);
    assert.equal(result.data?.overflow_tasks?.length, expected.overflow_count);
    assert.equal(result.data?.strategy_notes?.length, expected.strategy_note_count);
  });
});
