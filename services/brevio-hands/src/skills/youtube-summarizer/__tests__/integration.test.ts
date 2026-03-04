import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  video_id: string;
  key_points_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'summary-success.json'), 'utf8')) as Fixture;

describe('youtube-summarizer integration', () => {
  it('returns deterministic video summary payload', async () => {
    const result = await adapter.execute(
      { action: 'summarize_video', video_id: 'abc12345', max_points: 2 },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.video_id, expected.video_id);
    assert.equal(result.data?.key_points?.length, expected.key_points_count);
  });
});
