import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('swissweather adapter', () => {
  it('requires location', async () => {
    const result = await adapter.execute({ action: 'forecast' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /SWISSWEATHER_LOCATION_REQUIRED/);
  });

  it('returns deterministic forecast', async () => {
    const result = await adapter.execute({ action: 'forecast', location: 'Zurich' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'swissweather');
  });
});
