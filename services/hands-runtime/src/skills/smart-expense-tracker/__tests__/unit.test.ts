import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('smart-expense-tracker adapter', () => {
  it('requires fields for log expense', async () => {
    const result = await adapter.execute({ action: 'log_expense', merchant: 'Cafe' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /SMART_EXPENSE_TRACKER_LOG_FIELDS_REQUIRED/);
  });

  it('returns daily briefing data', async () => {
    const result = await adapter.execute({ action: 'daily_briefing' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'smart-expense-tracker');
    assert.ok(Array.isArray(result.data?.entries));
  });
});
