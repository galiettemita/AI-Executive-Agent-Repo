import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('food-delivery-ordering adapter', () => {
  it('requires confirmation for checkout', async () => {
    const result = await adapter.execute({ action: 'checkout', cart_id: 'cart_fd_001' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /FOOD_DELIVERY_CHECKOUT_CONFIRMATION_REQUIRED/);
  });

  it('returns restaurant options for address search', async () => {
    const result = await adapter.execute(
      { action: 'search_restaurants', address: '123 Main St, Austin, TX' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'food-delivery-ordering');
    assert.ok(Array.isArray(result.data?.restaurants));
  });
});
