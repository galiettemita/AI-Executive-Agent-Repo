import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('asr adapter', () => {
  it('validates required audio fields', async () => {
    const result = await adapter.execute({ mime_type: 'audio/ogg', duration_ms: 30000 }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /ASR_AUDIO_URL_REQUIRED|Required/);
  });

  it('returns transcript and segments', async () => {
    const result = await adapter.execute(
      {
        audio_url: 'https://cdn.example.com/audio/meeting-note.ogg',
        mime_type: 'audio/ogg',
        duration_ms: 30000,
        language_hint: 'en-US'
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'asr');
    assert.ok(Array.isArray(result.data?.segments));
  });
});
