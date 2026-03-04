import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('lastfm adapter', () => {
  it('requires username for recent tracks', async () => {
    const result = await adapter.execute({ action: 'recent_tracks' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /LASTFM_USERNAME_REQUIRED/);
  });

  it('returns top tracks with username', async () => {
    const result = await adapter.execute({ action: 'top_tracks', username: 'execuser' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'lastfm');
    assert.ok(Array.isArray(result.data?.tracks));
  });
});
