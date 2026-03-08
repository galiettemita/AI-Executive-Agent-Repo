import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('monarch-money adapter', () => {
  it('requires account_id for transactions action', async () => {
    const result = await adapter.execute({ action: 'transactions' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /MONARCH_MONEY_ACCOUNT_REQUIRED/);
  });

  it('returns monthly budgets', async () => {
    const result = await adapter.execute({ action: 'budgets', month: '2026-03' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'monarch-money');
    assert.ok(Array.isArray(result.data?.budgets));
  });
});
