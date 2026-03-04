import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('youtube-api unit', () => {
  it('returns search results', async () => {
    const output = await runClient({ mode: 'search', query: 'executive planning' });
    assert.equal(output.provider, 'youtube');
    assert.equal(output.mode, 'search');
    assert.ok((output.results?.length ?? 0) > 0);
  });

  it('returns transcript for known video', async () => {
    const output = await runClient({ mode: 'transcript', video_id: 'vid_exec_weekly_01' });
    assert.equal(output.mode, 'transcript');
    assert.ok((output.transcript?.length ?? 0) > 0);
  });
});
