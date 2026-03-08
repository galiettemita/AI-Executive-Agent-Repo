import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('apple-notes adapter', () => {
  it('requires title/body for create', async () => {
    const result = await adapter.execute({ action: 'create_note', title: 'Draft' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /APPLE_NOTES_CREATE_FIELDS_REQUIRED/);
  });

  it('returns alias metadata and notes list', async () => {
    const result = await adapter.execute({ action: 'list_recent' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'apple-notes');
    assert.equal(result.data?.canonical_skill_id, 'apple-notes-skill');
  });
});
