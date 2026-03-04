import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('last30days adapter', () => {
  it('requires query', async () => {
    const result = await adapter.execute({ action: 'scan_topic' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /LAST30DAYS_QUERY_REQUIRED/);
  });

  it('returns deterministic highlights', async () => {
    const result = await adapter.execute({ action: 'scan_topic', query: 'AI copilots' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'last30days');
  });
});
