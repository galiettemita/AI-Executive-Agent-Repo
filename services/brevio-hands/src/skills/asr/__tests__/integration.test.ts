import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

const testDir = dirname(fileURLToPath(import.meta.url));

interface AsrFixture {
  transcript: string;
  language: string;
  confidence: number;
  segment_count: number;
  latency_budget_ms: number;
}

function readFixture(name: string): AsrFixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', name), 'utf8')) as AsrFixture;
}

describe('asr integration', () => {
  it('returns fixture-backed transcript metadata', async () => {
    const result = await adapter.execute(
      {
        audio_url: 'https://media.brevio.local/audio/meeting-note.wav',
        mime_type: 'audio/wav',
        duration_ms: 30000
      },
      {} as never
    );

    const expected = readFixture('meeting-success.json');
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.transcript, expected.transcript);
    assert.equal(result.data?.language, expected.language);
    assert.equal(result.data?.confidence, expected.confidence);
    assert.equal(result.data?.latency_budget_ms, expected.latency_budget_ms);
    assert.equal(result.data?.segments?.length, expected.segment_count);
  });
});
