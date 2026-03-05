import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('flight-tracker adapter', () => {
  it('rejects missing identifiers', async () => {
    const result = await adapter.execute({}, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.equal(result.error?.code, 'VALIDATION_FAILED');
    assert.match(result.error?.message ?? '', /FLIGHT_TRACKER_IDENTIFIER_REQUIRED/);
  });

  it('returns flights for callsign filter', async () => {
    const result = await adapter.execute({ callsign: 'AAL100' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'opensky');
    assert.equal(result.data?.flights?.[0]?.callsign, 'AAL100');
  });
});
