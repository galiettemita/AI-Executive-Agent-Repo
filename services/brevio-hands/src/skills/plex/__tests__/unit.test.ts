import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('plex adapter', () => {
  it('validates query requirement for search', async () => {
    const result = await adapter.execute({ action: 'search' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /PLEX_QUERY_REQUIRED/);
  });

  it('returns recent library items', async () => {
    const result = await adapter.execute({ action: 'recent' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'plex');
    assert.ok(Array.isArray(result.data?.results));
  });
});
