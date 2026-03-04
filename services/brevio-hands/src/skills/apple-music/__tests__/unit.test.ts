import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('apple-music adapter', () => {
  it('validates required search query', async () => {
    const result = await adapter.execute({ action: 'search' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /APPLE_MUSIC_QUERY_REQUIRED/);
  });

  it('returns tracks for query search', async () => {
    const result = await adapter.execute({ action: 'search', query: 'focus' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'apple-music');
    assert.ok(Array.isArray(result.data?.tracks));
  });
});
