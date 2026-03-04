import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('bluesky adapter', () => {
  it('requires confirmation for post action', async () => {
    const result = await adapter.execute({ action: 'post', text: 'New update', confirmed: false }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /BLUESKY_POST_CONFIRMATION_REQUIRED/);
  });

  it('returns timeline posts', async () => {
    const result = await adapter.execute({ action: 'timeline' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'bluesky');
    assert.ok(Array.isArray(result.data?.posts));
  });
});
