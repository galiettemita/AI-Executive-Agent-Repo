import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('goplaces adapter', () => {
  it('rejects short queries', async () => {
    const result = await adapter.execute({ query: 'a' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.equal(result.error?.code, 'VALIDATION_FAILED');
  });

  it('returns place candidates for valid query', async () => {
    const result = await adapter.execute({ query: 'coffee', open_now: true }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'goplaces');
    assert.ok(Array.isArray(result.data?.results));
  });
});
