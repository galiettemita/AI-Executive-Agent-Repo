import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

const testDir = dirname(fileURLToPath(import.meta.url));

interface GeminiFixture {
  transcript: string;
  language: string;
  confidence: number;
  speaker_count: number;
  latency_budget_ms: number;
}

function readFixture(name: string): GeminiFixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', name), 'utf8')) as GeminiFixture;
}

describe('gemini-stt integration', () => {
  it('returns fixture-backed speaker transcript metadata', async () => {
    const result = await adapter.execute(
      {
        audio_url: 'https://media.brevio.local/audio/interview.wav',
        duration_ms: 60000,
        include_speaker_labels: true
      },
      {} as never
    );

    const expected = readFixture('interview-success.json');
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.transcript, expected.transcript);
    assert.equal(result.data?.language, expected.language);
    assert.equal(result.data?.confidence, expected.confidence);
    assert.equal(result.data?.latency_budget_ms, expected.latency_budget_ms);
    assert.equal(result.data?.speakers?.length, expected.speaker_count);
  });
});
