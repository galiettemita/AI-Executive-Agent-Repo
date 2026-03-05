import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('daily-rhythm adapter', () => {
  it('validates required compose context', async () => {
    const result = await adapter.execute(
      {
        action: 'compose_briefing',
        timezone: 'America/New_York',
        date: '2026-03-04'
      },
      {} as never
    );
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /DAILY_RHYTHM_WAKE_TIME_REQUIRED/);
  });

  it('builds a morning briefing response', async () => {
    const result = await adapter.execute(
      {
        action: 'compose_briefing',
        timezone: 'America/New_York',
        date: '2026-03-04',
        wake_time_local: '07:00',
        weather_summary: 'Light rain, high of 62F',
        tasks: [
          { title: 'Finalize board deck', priority: 'high', estimated_minutes: 90 },
          { title: 'Review hiring packet', priority: 'medium', estimated_minutes: 45 }
        ]
      },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'daily-rhythm');
    assert.ok(Array.isArray(result.data?.schedule_blocks));
  });
});
