import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('ride-hailing adapter', () => {
  it('requires confirmation for ride request', async () => {
    const result = await adapter.execute(
      { action: 'request_ride', origin: 'A', destination: 'B' },
      {} as never
    );
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /RIDE_HAILING_CONFIRMATION_REQUIRED/);
  });

  it('returns ride estimates for route', async () => {
    const result = await adapter.execute(
      { action: 'estimate', origin: 'Downtown', destination: 'Airport' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'ride-hailing');
    assert.ok(Array.isArray(result.data?.estimates));
  });
});
