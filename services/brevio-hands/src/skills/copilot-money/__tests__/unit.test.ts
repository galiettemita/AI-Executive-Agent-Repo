import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('copilot-money adapter', () => {
  it('requires account_id for transaction queries', async () => {
    const result = await adapter.execute({ action: 'transactions' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /COPILOT_MONEY_ACCOUNT_REQUIRED/);
  });

  it('returns net worth summary', async () => {
    const result = await adapter.execute({ action: 'net_worth' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'copilot-money');
    assert.equal(typeof result.data?.net_worth_cents, 'number');
  });
});
