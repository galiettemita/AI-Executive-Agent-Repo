import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('spotify adapter', () => {
  it('requires query for play action', async () => {
    const result = await adapter.execute({ action: 'play' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /SPOTIFY_QUERY_REQUIRED/);
  });

  it('returns playback status', async () => {
    const result = await adapter.execute({ action: 'status' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'spotify');
    assert.equal(typeof result.data?.now_playing.track, 'string');
  });
});
