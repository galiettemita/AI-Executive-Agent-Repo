import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('veo adapter', () => {
  it('requires prompt for generate action', async () => {
    const result = await adapter.execute({ action: 'generate_video' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /VEO_PROMPT_REQUIRED/);
  });

  it('returns deterministic job payload', async () => {
    const result = await adapter.execute({ action: 'generate_video', prompt: 'A drone flyover of mountains' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'veo');
  });
});
