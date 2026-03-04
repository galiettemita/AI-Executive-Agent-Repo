import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('sag adapter', () => {
  it('requires text input', async () => {
    const result = await adapter.execute({}, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /SAG_TEXT_REQUIRED|Required/);
  });

  it('returns premium voice output metadata', async () => {
    const result = await adapter.execute(
      { text: 'Your car is on the way.', style: 'energetic' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'sag');
    assert.match(result.data?.audio_url ?? '', /^https:\/\//);
  });
});
