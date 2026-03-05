import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('financial-market-analysis adapter', () => {
  it('requires symbols input', async () => {
    const result = await adapter.execute({ action: 'sentiment', symbols: [] }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /FINANCIAL_MARKET_ANALYSIS_SYMBOLS_REQUIRED/);
  });

  it('returns correlation matrix', async () => {
    const result = await adapter.execute(
      { action: 'correlation', symbols: ['AAPL', 'MSFT', 'TSLA'] },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'financial-market-analysis');
    assert.ok(Array.isArray(result.data?.correlation?.matrix));
  });
});
