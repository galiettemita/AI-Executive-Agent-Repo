import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('apple-photos adapter', () => {
  it('requires query for search', async () => {
    const result = await adapter.execute({ action: 'search_photos' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /APPLE_PHOTOS_QUERY_REQUIRED/);
  });

  it('lists recent photos', async () => {
    const result = await adapter.execute({ action: 'recent_photos', limit: 2 }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'apple-photos');
    assert.ok(Array.isArray(result.data?.photos));
  });
});
