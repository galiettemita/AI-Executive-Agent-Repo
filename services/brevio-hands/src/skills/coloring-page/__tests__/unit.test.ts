import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('coloring-page adapter', () => {
  it('requires prompt for generate_from_prompt', async () => {
    const result = await adapter.execute({ action: 'generate_from_prompt' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /COLORING_PAGE_PROMPT_REQUIRED/);
  });

  it('returns deterministic output metadata', async () => {
    const result = await adapter.execute({ action: 'generate_from_prompt', prompt: 'A happy dragon' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'coloring-page');
  });
});
