import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('content-advisory adapter', () => {
  it('requires title', async () => {
    const result = await adapter.execute({ action: 'evaluate_title' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /CONTENT_ADVISORY_TITLE_REQUIRED/);
  });

  it('returns advisory categories', async () => {
    const result = await adapter.execute({ action: 'evaluate_title', title: 'Example Film' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'content-advisory');
  });
});
