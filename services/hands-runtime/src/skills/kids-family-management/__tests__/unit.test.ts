import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('kids-family-management adapter', () => {
  it('requires child and date for pickup planning', async () => {
    const result = await adapter.execute({ action: 'pickup_plan', child_name: 'Mia' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /KIDS_FAMILY_PICKUP_FIELDS_REQUIRED/);
  });

  it('returns family schedule', async () => {
    const result = await adapter.execute({ action: 'family_schedule' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.ok(Array.isArray(result.data?.events));
  });
});
