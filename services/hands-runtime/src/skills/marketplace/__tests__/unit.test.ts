import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('marketplace adapter', () => {
  it('requires title for evaluate_listing', async () => {
    const result = await adapter.execute({ action: 'evaluate_listing' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /MARKETPLACE_TITLE_REQUIRED/);
  });

  it('evaluates fair price', async () => {
    const result = await adapter.execute(
      {
        action: 'compare_prices',
        title: 'Used office chair',
        comparable_prices_cents: [12000, 15000, 13500]
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'marketplace');
    assert.equal(typeof result.data?.fair_price_cents, 'number');
  });
});
