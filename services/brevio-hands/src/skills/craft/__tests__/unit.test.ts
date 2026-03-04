import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('craft adapter', () => {
  it('requires title for create_doc', async () => {
    const result = await adapter.execute({ action: 'create_doc' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /CRAFT_DOC_TITLE_REQUIRED/);
  });

  it('returns doc metadata', async () => {
    const result = await adapter.execute({ action: 'create_doc', doc_title: 'Weekly Notes' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'craft');
  });
});
