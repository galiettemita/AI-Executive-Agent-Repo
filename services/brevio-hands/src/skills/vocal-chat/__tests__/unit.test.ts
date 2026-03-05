import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('vocal-chat adapter', () => {
  it('requires voice payload fields', async () => {
    const result = await adapter.execute({}, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /VOCAL_CHAT_AUDIO_REQUIRED|Required/);
  });

  it('returns round-trip voice metadata', async () => {
    const result = await adapter.execute(
      {
        audio_url: 'https://cdn.example.com/audio/schedule-request.ogg',
        mime_type: 'audio/ogg',
        duration_ms: 32000,
        response_voice: 'alloy'
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'vocal-chat');
    assert.match(result.data?.reply_audio_url ?? '', /^https:\/\//);
  });
});
