import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('plaid unit', () => {
  it('returns linked accounts', async () => {
    const output = await runClient({ action: 'accounts' });
    assert.equal(output.provider, 'plaid');
    assert.ok((output.accounts?.length ?? 0) > 0);
  });

  it('returns filtered balances', async () => {
    const output = await runClient({ action: 'balance', account_id: 'plaid_checking' });
    assert.equal(output.action, 'balance');
    assert.ok(output.balances?.every((balance) => balance.account_id === 'plaid_checking'));
  });
});
