import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('gemini-stt adapter', () => {
  it('validates required fields', async () => {
    const result = await adapter.execute({ duration_ms: 45000 }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /GEMINI_STT_AUDIO_URL_REQUIRED|Required/);
  });

  it('returns transcript with speaker metadata', async () => {
    const result = await adapter.execute(
      {
        audio_url: 'https://cdn.example.com/audio/interview.wav',
        duration_ms: 45000,
        include_speaker_labels: true
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'gemini-stt');
    assert.ok(Array.isArray(result.data?.speakers));
  });
});
