import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('omnifocus adapter', () => {
  it('requires title for add_task', async () => {
    const result = await adapter.execute({ action: 'add_task' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /OMNIFOCUS_TITLE_REQUIRED/);
  });

  it('returns deterministic flagged tasks', async () => {
    const result = await adapter.execute({ action: 'list_flagged' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'omnifocus');
    assert.equal(typeof result.data?.flagged_count, 'number');
  });
});
