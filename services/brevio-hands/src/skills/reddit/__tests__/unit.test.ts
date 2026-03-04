import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('reddit adapter', () => {
  it('requires confirmation for post', async () => {
    const result = await adapter.execute(
      {
        action: 'post',
        subreddit: 'operations',
        title: 'Weekly review format',
        text: 'Sharing a template',
        confirmed: false
      },
      {} as never
    );
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /REDDIT_POST_CONFIRMATION_REQUIRED/);
  });

  it('searches posts', async () => {
    const result = await adapter.execute({ action: 'search', query: 'prompt' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'reddit');
    assert.ok(Array.isArray(result.data?.posts));
  });
});
