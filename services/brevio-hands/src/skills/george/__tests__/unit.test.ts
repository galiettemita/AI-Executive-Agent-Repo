import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('george adapter', () => {
  it('requires account id for transaction analysis', async () => {
    const result = await adapter.execute({ action: 'analyze_transactions' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /GEORGE_ACCOUNT_REQUIRED/);
  });

  it('returns account snapshot', async () => {
    const result = await adapter.execute({ action: 'fetch_accounts' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'george');
  });
});
