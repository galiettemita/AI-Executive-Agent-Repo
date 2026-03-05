import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('resume-builder adapter', () => {
  it('requires role for generate action', async () => {
    const result = await adapter.execute({ action: 'generate' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /RESUME_BUILDER_ROLE_REQUIRED/);
  });

  it('returns resume score recommendations', async () => {
    const result = await adapter.execute(
      { action: 'score', resume_markdown: '# Resume\n\n- Bullet 1\n- Bullet 2' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'resume-builder');
    assert.equal(typeof result.data?.score, 'number');
  });
});
