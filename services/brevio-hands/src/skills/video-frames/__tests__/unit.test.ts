import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('video-frames adapter', () => {
  it('requires timestamp for single frame', async () => {
    const result = await adapter.execute(
      { action: 'extract_frame', video_url: 'https://example.com/video.mp4' },
      {} as never
    );
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /VIDEO_FRAMES_TIMESTAMP_REQUIRED/);
  });

  it('extracts batch frame URLs', async () => {
    const result = await adapter.execute(
      {
        action: 'extract_frames',
        video_url: 'https://example.com/video.mp4',
        frame_interval_seconds: 10,
        frame_count: 3
      },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'video-frames');
    assert.equal(result.data?.extracted_count, 3);
  });
});
