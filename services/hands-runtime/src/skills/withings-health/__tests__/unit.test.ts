import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('withings-health adapter', () => {
  it('requires measure type', async () => {
    const result = await adapter.execute({ action: 'get_measurements' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /WITHINGS_MEASURE_TYPE_REQUIRED/);
  });

  it('returns trend summary', async () => {
    const result = await adapter.execute(
      {
        action: 'trend_summary',
        measure_type: 'weight',
        start_date: '2026-03-02',
        end_date: '2026-03-04'
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'withings-health');
    assert.ok(Array.isArray(result.data?.measurements));
  });
});
