import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('sleep-calculator adapter', () => {
  it('requires wake time for bedtime calculation', async () => {
    const result = await adapter.execute({ action: 'bedtime_from_wake' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /SLEEP_CALCULATOR_WAKE_TIME_REQUIRED/);
  });

  it('returns bedtime recommendations', async () => {
    const result = await adapter.execute(
      {
        action: 'bedtime_from_wake',
        wake_time_local: '06:30'
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'sleep-calculator');
    assert.ok(Array.isArray(result.data?.recommendations));
  });
});
