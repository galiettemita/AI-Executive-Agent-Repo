import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('aerobase-skill adapter', () => {
  it('requires route fields', async () => {
    const result = await adapter.execute({ action: 'search_flights' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /AEROBASE_ROUTE_REQUIRED/);
  });

  it('returns itinerary list', async () => {
    const result = await adapter.execute(
      { action: 'search_flights', origin: 'JFK', destination: 'LHR', depart_date: '2026-04-10' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'aerobase-skill');
  });
});
