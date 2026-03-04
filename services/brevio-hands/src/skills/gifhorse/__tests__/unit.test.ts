import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('gifhorse adapter', () => {
  it('requires query', async () => {
    const result = await adapter.execute({ action: 'search_gif' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /GIFHORSE_QUERY_REQUIRED/);
  });

  it('returns deterministic gif result', async () => {
    const result = await adapter.execute({ action: 'search_gif', query: 'celebration' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'gifhorse');
  });
});
