import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('voice-wake-say adapter', () => {
  it('requires text input', async () => {
    const result = await adapter.execute({}, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /VOICE_WAKE_SAY_TEXT_REQUIRED|Required/);
  });

  it('returns local say command metadata', async () => {
    const result = await adapter.execute(
      { text: 'Meeting starts in ten minutes.', voice: 'Samantha', rate_wpm: 200 },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'voice-wake-say');
    assert.match(result.data?.command ?? '', /^say -v/);
  });
});
