import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('aviationstack-flight-tracker adapter', () => {
  it('rejects missing identifiers', async () => {
    const result = await adapter.execute({}, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.equal(result.error?.code, 'VALIDATION_FAILED');
    assert.match(result.error?.message ?? '', /AVIATIONSTACK_FLIGHT_IDENTIFIER_REQUIRED/);
  });

  it('returns premium flight detail records', async () => {
    const result = await adapter.execute({ flight_iata: 'AA100' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'aviationstack');
    assert.equal(result.data?.flights?.[0]?.flight, 'AA100');
  });
});
