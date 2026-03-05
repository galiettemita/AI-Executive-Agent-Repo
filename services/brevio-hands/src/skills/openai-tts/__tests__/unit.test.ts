import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('openai-tts adapter', () => {
  it('requires text', async () => {
    const result = await adapter.execute({}, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /OPENAI_TTS_TEXT_REQUIRED|Required/);
  });

  it('returns synthesized audio metadata', async () => {
    const result = await adapter.execute(
      { text: 'Your briefing is ready.', voice: 'alloy', format: 'mp3' },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'openai-tts');
    assert.match(result.data?.audio_url ?? '', /^https:\/\//);
  });
});
