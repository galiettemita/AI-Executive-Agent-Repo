import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('youtube-summarizer adapter', () => {
  it('requires video id or url', async () => {
    const result = await adapter.execute({ action: 'summarize_video' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /YOUTUBE_SUMMARIZER_VIDEO_REQUIRED/);
  });

  it('returns summary and key points', async () => {
    const result = await adapter.execute(
      { action: 'key_points', video_id: 'abc123xyz', max_points: 2 },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'youtube-summarizer');
    assert.equal((result.data?.key_points ?? []).length, 2);
  });
});
