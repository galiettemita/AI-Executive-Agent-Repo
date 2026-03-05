import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('clawringhouse adapter', () => {
  it('requires items for proactive recommendations', async () => {
    const result = await adapter.execute({ action: 'proactive_recommendations' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /CLAWRINGHOUSE_ITEMS_REQUIRED/);
  });

  it('creates reorder recommendations', async () => {
    const result = await adapter.execute(
      {
        action: 'detect_reorder_need',
        household_items: [
          { name: 'Coffee beans', days_since_last_order: 24, typical_cycle_days: 21, estimated_units_left: 0 },
          { name: 'Dish soap', days_since_last_order: 8, typical_cycle_days: 30, estimated_units_left: 2 }
        ]
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'clawringhouse');
    assert.ok(Array.isArray(result.data?.recommendations));
  });
});
