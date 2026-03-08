import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('things-mac adapter', () => {
  it('requires title for create_todo', async () => {
    const result = await adapter.execute({ action: 'create_todo' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /THINGS_MAC_TITLE_REQUIRED/);
  });

  it('returns deterministic list_today payload', async () => {
    const result = await adapter.execute({ action: 'list_today' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'things-mac');
    assert.equal(Array.isArray(result.data?.todos), true);
  });
});
