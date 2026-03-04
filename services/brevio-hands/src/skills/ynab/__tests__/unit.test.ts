import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('ynab unit', () => {
  it('returns budget summary', async () => {
    const output = await runClient({ action: 'summary' });
    assert.equal(output.provider, 'ynab');
    assert.equal(output.action, 'summary');
    assert.ok((output.total_budget_cents ?? 0) > 0);
  });

  it('returns filtered transactions', async () => {
    const output = await runClient({
      action: 'transactions',
      account_id: 'acct_checking'
    });

    assert.equal(output.action, 'transactions');
    assert.ok((output.transactions?.length ?? 0) > 0);
    assert.ok(output.transactions?.every((txn) => txn.account_id === 'acct_checking'));
  });
});
