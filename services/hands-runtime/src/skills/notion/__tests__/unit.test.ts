import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('notion unit', () => {
  it('searches pages', async () => {
    const output = await runClient({ action: 'search', query: 'executive' });
    assert.equal(output.provider, 'notion');
    assert.equal(output.action, 'search');
    assert.ok((output.pages?.length ?? 0) > 0);
  });

  it('creates page', async () => {
    const output = await runClient({
      action: 'create_page',
      title: 'Board Prep Notes',
      content: 'Agenda and priorities'
    });

    assert.equal(output.action, 'create_page');
    assert.ok(output.page_id?.startsWith('page_'));
  });
});
