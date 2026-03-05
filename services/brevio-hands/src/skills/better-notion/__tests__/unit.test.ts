import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('better-notion adapter', () => {
  it('requires title for create_page', async () => {
    const result = await adapter.execute({ action: 'create_page' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /BETTER_NOTION_CREATE_TITLE_REQUIRED/);
  });

  it('returns page metadata', async () => {
    const result = await adapter.execute({ action: 'create_page', page_title: 'Roadmap' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'better-notion');
  });
});
