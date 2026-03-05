import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('krea-api adapter', () => {
  it('requires prompt for generate_image', async () => {
    const result = await adapter.execute({ action: 'generate_image' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /KREA_API_PROMPT_REQUIRED/);
  });

  it('returns deterministic image metadata', async () => {
    const result = await adapter.execute({ action: 'generate_image', prompt: 'Architectural rendering' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'krea-api');
  });
});
