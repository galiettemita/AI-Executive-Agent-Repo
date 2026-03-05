import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  segment_count: number;
  has_subtitle_url: boolean;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'subtitles-success.json'), 'utf8')) as Fixture;

describe('video-transcript-downloader integration', () => {
  it('returns deterministic transcript/subtitle payload', async () => {
    const result = await adapter.execute(
      {
        action: 'fetch_subtitles',
        video_id: 'abc12345',
        language: 'en'
      },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.segment_count, expected.segment_count);
    assert.equal(typeof result.data?.subtitle_url === 'string', expected.has_subtitle_url);
  });
});
