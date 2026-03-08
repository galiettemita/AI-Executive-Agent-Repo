import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('spotify-player adapter', () => {
  it('requires query for search', async () => {
    const result = await adapter.execute({ action: 'search_tracks' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /SPOTIFY_PLAYER_QUERY_REQUIRED/);
  });

  it('returns playback queue snapshot', async () => {
    const result = await adapter.execute({ action: 'playback_status' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'spotify-player');
    assert.equal(typeof result.data?.queue_length, 'number');
  });
});
