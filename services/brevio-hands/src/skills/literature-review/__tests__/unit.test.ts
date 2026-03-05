import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('literature-review adapter', () => {
  it('requires topic', async () => {
    const result = await adapter.execute({ action: 'search_papers' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /LITERATURE_REVIEW_TOPIC_REQUIRED/);
  });

  it('returns deterministic paper list', async () => {
    const result = await adapter.execute({ action: 'search_papers', topic: 'agentic workflows' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'literature-review');
  });
});
