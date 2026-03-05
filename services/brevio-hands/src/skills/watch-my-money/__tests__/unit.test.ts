import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('watch-my-money adapter', () => {
  it('requires transactions and income', async () => {
    const result = await adapter.execute({ action: 'analyze_statement' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /WATCH_MY_MONEY_TRANSACTIONS_REQUIRED/);
  });

  it('returns spend analysis', async () => {
    const result = await adapter.execute(
      {
        action: 'budget_alerts',
        monthly_income_cents: 600000,
        transactions: [
          { merchant: 'Grocer', amount_cents: 22000, category: 'Groceries' },
          { merchant: 'Cafe', amount_cents: 9500, category: 'Dining' }
        ]
      },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'watch-my-money');
    assert.equal(typeof result.data?.spend_rate_pct_of_income, 'number');
  });
});
