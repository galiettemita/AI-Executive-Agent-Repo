import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('self-improvement adapter', () => {
  it('requires lesson text', async () => {
    const result = await adapter.execute({ action: 'log_lesson' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /SELF_IMPROVEMENT_LESSON_REQUIRED/);
  });

  it('returns deterministic next steps', async () => {
    const result = await adapter.execute(
      { action: 'log_lesson', lesson: 'I overcommitted this week and need clearer boundaries.' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'self-improvement');
  });
});
