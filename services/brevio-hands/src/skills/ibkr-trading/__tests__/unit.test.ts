import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('ibkr-trading adapter', () => {
  it('requires symbol for quote action', async () => {
    const result = await adapter.execute({ action: 'quote_symbol' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /IBKR_TRADING_SYMBOL_REQUIRED/);
  });

  it('returns quote payload', async () => {
    const result = await adapter.execute({ action: 'quote_symbol', symbol: 'AAPL' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'ibkr-trading');
  });
});
