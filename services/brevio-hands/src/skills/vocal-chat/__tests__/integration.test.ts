import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(name: string): unknown {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', name), 'utf8')) as unknown;
}

describe('vocal-chat integration', () => {
  it('returns fixture-backed voice round-trip payload', async () => {
    const result = await adapter.execute(
      {
        audio_url: 'https://media.brevio.local/audio/schedule-voice-note.wav',
        mime_type: 'audio/wav',
        duration_ms: 38000,
        response_voice: 'alloy'
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.deepEqual(result.data, readFixture('round-trip-success.json'));
  });
});
