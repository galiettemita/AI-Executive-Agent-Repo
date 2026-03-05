import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('expense-tracker-pro adapter', () => {
  it('requires complete add fields', async () => {
    const result = await adapter.execute({ action: 'add_expense', merchant: 'Cafe' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /EXPENSE_TRACKER_PRO_ADD_FIELDS_REQUIRED/);
  });

  it('returns monthly summary totals', async () => {
    const result = await adapter.execute({ action: 'monthly_summary', month: '2026-03' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'expense-tracker-pro');
    assert.equal(typeof result.data?.total_cents, 'number');
  });
});
