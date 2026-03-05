import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('buy-anything adapter', () => {
  it('requires confirmation for place_order', async () => {
    const result = await adapter.execute(
      {
        action: 'place_order',
        shipping_address_id: 'addr_001',
        line_items: [{ sku: 'AMZ-RUN-001', title: 'Carbon Running Shoe', quantity: 1, unit_price_cents: 12999 }]
      },
      {} as never
    );

    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /BUY_ANYTHING_ORDER_CONFIRMATION_REQUIRED/);
  });

  it('searches product options', async () => {
    const result = await adapter.execute({ action: 'search_product', query: 'running shoes' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'buy-anything');
    assert.ok(Array.isArray(result.data?.product_options));
  });
});
