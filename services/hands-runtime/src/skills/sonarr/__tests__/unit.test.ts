import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('sonarr adapter', () => {
  it('requires query for search action', async () => {
    const result = await adapter.execute({ action: 'search_series' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /SONARR_QUERY_REQUIRED/);
  });

  it('returns deterministic queue snapshot', async () => {
    const result = await adapter.execute({ action: 'list_queue' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'sonarr');
    assert.equal(typeof result.data?.queue_count, 'number');
  });
});
