import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('local-places adapter', () => {
  it('rejects short queries', async () => {
    const result = await adapter.execute({ query: 'a' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.equal(result.error?.code, 'VALIDATION_FAILED');
  });

  it('returns nearby places for valid query', async () => {
    const result = await adapter.execute({ query: 'coffee', radius_km: 5 }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'local-places');
    assert.ok(Array.isArray(result.data?.results));
  });
});
