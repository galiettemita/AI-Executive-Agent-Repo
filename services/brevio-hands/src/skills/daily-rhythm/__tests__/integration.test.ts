import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  priorities: string[];
  schedule_block_count: number;
  nudges_count: number;
  briefing_contains: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'compose-success.json'), 'utf8')) as Fixture;
}

describe('daily-rhythm integration', () => {
  it('returns deterministic daily briefing payload', async () => {
    const result = await adapter.execute(
      {
        action: 'compose_briefing',
        timezone: 'America/New_York',
        date: '2026-03-05',
        wake_time_local: '07:00',
        weather_summary: 'Cloudy',
        energy_level: 'steady',
        tasks: [
          { title: 'Deep Strategy Block', priority: 'high', estimated_minutes: 90 },
          { title: 'Admin Sweep', priority: 'low', estimated_minutes: 30 }
        ],
        meetings: [{ title: 'Exec Sync', start_local: '10:00', end_local: '10:30' }]
      },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.deepEqual(result.data?.priorities, expected.priorities);
    assert.equal(result.data?.schedule_blocks?.length, expected.schedule_block_count);
    assert.equal(result.data?.nudges?.length, expected.nudges_count);
    assert.match(result.data?.briefing_text ?? '', new RegExp(expected.briefing_contains));
  });
});
