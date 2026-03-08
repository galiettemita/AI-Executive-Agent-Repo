import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('restaurant-reservations adapter', () => {
  it('requires confirmation before booking', async () => {
    const result = await adapter.execute(
      { action: 'book', hold_id: 'hold_rest_001', confirmed: false },
      {} as never
    );
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /RESTAURANT_RESERVATIONS_CONFIRMATION_REQUIRED/);
  });

  it('returns options on search', async () => {
    const result = await adapter.execute(
      { action: 'search', city: 'Austin', date: '2026-03-10', party_size: 2 },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'restaurant-reservations');
    assert.ok(Array.isArray(result.data?.options));
  });
});
