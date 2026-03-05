import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('dexcom adapter', () => {
  it('requires range or window input', async () => {
    const result = await adapter.execute({ action: 'glucose_readings' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /DEXCOM_TIME_RANGE_REQUIRED/);
  });

  it('returns glucose readings', async () => {
    const result = await adapter.execute(
      {
        action: 'trend_alerts',
        minutes: 30
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'dexcom');
    assert.ok(Array.isArray(result.data?.readings));
  });
});
