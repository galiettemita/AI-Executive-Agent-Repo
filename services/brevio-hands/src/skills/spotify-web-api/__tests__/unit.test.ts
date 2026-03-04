import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('spotify-web-api unit', () => {
  it('returns playback snapshot', async () => {
    const output = await runClient({ action: 'playback' });
    assert.equal(output.provider, 'spotify-web-api');
    assert.equal(output.action, 'playback');
    assert.ok((output.playing?.progress_ms ?? 0) > 0);
  });

  it('searches tracks', async () => {
    const output = await runClient({ action: 'search', query: 'focus' });
    assert.equal(output.action, 'search');
    assert.ok((output.results?.length ?? 0) > 0);
  });
});
