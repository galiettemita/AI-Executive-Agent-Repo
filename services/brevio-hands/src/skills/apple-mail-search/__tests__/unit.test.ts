import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('apple-mail-search adapter', () => {
  it('requires query input', async () => {
    const result = await adapter.execute({ action: 'search_all' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /APPLE_MAIL_SEARCH_QUERY_REQUIRED/);
  });

  it('returns indexed mail results', async () => {
    const result = await adapter.execute(
      { action: 'search_subject', query: 'board', limit: 5 },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'apple-mail-search');
    assert.equal(result.data?.latency_profile_ms, 50);
  });
});
