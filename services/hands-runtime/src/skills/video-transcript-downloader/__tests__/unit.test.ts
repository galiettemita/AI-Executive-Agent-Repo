import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('video-transcript-downloader adapter', () => {
  it('requires video id or url', async () => {
    const result = await adapter.execute({ action: 'fetch_transcript' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /VIDEO_TRANSCRIPT_VIDEO_REQUIRED/);
  });

  it('returns transcript payload', async () => {
    const result = await adapter.execute(
      { action: 'fetch_subtitles', video_id: 'abc123xyz', language: 'en' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'video-transcript-downloader');
    assert.equal(typeof result.data?.segment_count, 'number');
  });
});
