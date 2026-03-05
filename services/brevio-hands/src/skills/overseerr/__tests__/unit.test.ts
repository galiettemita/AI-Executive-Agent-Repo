import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('overseerr adapter', () => {
  it('requires query for search', async () => {
    const result = await adapter.execute({ action: 'search_media' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /OVERSEERR_QUERY_REQUIRED/);
  });

  it('returns deterministic request list', async () => {
    const result = await adapter.execute({ action: 'list_requests' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'overseerr');
    assert.equal(Array.isArray(result.data?.requests), true);
  });
});
