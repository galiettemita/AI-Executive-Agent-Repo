import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('spots adapter', () => {
  it('rejects short queries', async () => {
    const result = await adapter.execute({ query: 'a' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.equal(result.error?.code, 'VALIDATION_FAILED');
  });

  it('returns exhaustive place scan results', async () => {
    const result = await adapter.execute({ query: 'coffee', grid_density: 'high' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'spots');
    assert.equal(result.data?.grid_density, 'high');
  });
});
