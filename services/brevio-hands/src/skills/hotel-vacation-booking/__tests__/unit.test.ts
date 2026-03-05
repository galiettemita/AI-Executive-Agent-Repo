import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('hotel-vacation-booking adapter', () => {
  it('requires confirmation for booking', async () => {
    const result = await adapter.execute({ action: 'book_room', hold_id: 'hold_hotel_001' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /HOTEL_BOOKING_CONFIRMATION_REQUIRED/);
  });

  it('returns hotel options for search', async () => {
    const result = await adapter.execute(
      {
        action: 'search_hotels',
        city: 'Austin',
        check_in: '2026-04-01',
        check_out: '2026-04-03',
        guests: 2
      },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'hotel-vacation-booking');
    assert.ok(Array.isArray(result.data?.hotels));
  });
});
