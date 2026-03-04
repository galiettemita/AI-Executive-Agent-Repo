import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('yahoo-finance adapter', () => {
  it('requires symbols for quotes action', async () => {
    const result = await adapter.execute({ action: 'quotes' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /YAHOO_FINANCE_SYMBOLS_REQUIRED/);
  });

  it('returns market news with disclaimer', async () => {
    const result = await adapter.execute({ action: 'news' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'yahoo-finance');
    assert.match(result.data?.disclaimer ?? '', /Not financial advice/);
  });
});
