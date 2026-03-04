import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('pocket-casts adapter', () => {
  it('requires youtube url when queueing from youtube', async () => {
    const result = await adapter.execute({ action: 'queue_from_youtube' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /POCKET_CASTS_YOUTUBE_URL_REQUIRED/);
  });

  it('lists queue entries', async () => {
    const result = await adapter.execute({ action: 'list_queue' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'pocket-casts');
    assert.ok(Array.isArray(result.data?.queue));
  });
});
