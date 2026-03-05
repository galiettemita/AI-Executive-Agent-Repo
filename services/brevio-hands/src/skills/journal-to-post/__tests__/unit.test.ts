import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('journal-to-post adapter', () => {
  it('requires journal_entry', async () => {
    const result = await adapter.execute({ action: 'draft_post' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /JOURNAL_TO_POST_ENTRY_REQUIRED/);
  });

  it('returns deterministic post draft', async () => {
    const result = await adapter.execute(
      { action: 'draft_post', journal_entry: 'Today I learned to ship in smaller batches and review faster.' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'journal-to-post');
  });
});
