import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('pollinations adapter', () => {
  it('requires prompt', async () => {
    const result = await adapter.execute({ action: 'generate_image' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /POLLINATIONS_PROMPT_REQUIRED/);
  });

  it('returns deterministic asset metadata', async () => {
    const result = await adapter.execute({ action: 'generate_image', prompt: 'Neon skyline' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'pollinations');
  });
});
