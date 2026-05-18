import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('meal-planner adapter', () => {
  it('requires household size', async () => {
    const result = await adapter.execute({ action: 'weekly_plan' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /MEAL_PLANNER_HOUSEHOLD_SIZE_REQUIRED/);
  });

  it('returns weekly plan and grocery rollup', async () => {
    const result = await adapter.execute(
      {
        action: 'grocery_rollup',
        household_size: 2,
        dietary_preferences: ['high-protein'],
        calorie_target_per_person: 2300
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'meal-planner');
    assert.ok(Array.isArray(result.data?.grocery_items));
  });
});
