import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('ytmusic adapter', () => {
  it('validates required search query', async () => {
    const result = await adapter.execute({ action: 'search' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /YTMUSIC_QUERY_REQUIRED/);
  });

  it('returns tracks for search', async () => {
    const result = await adapter.execute({ action: 'search', query: 'calm' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'ytmusic');
    assert.ok(Array.isArray(result.data?.tracks));
  });
});
