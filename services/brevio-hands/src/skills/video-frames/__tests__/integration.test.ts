import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  extracted_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'extract-success.json'), 'utf8')) as Fixture;

describe('video-frames integration', () => {
  it('returns deterministic frame extraction payload', async () => {
    const result = await adapter.execute(
      {
        action: 'extract_frames',
        video_url: 'https://video.example.com/watch.mp4',
        frame_interval_seconds: 2,
        frame_count: 3
      },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.extracted_count, expected.extracted_count);
  });
});
