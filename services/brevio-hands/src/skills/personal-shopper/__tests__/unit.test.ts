import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('personal-shopper adapter', () => {
  it('requires query for research action', async () => {
    const result = await adapter.execute({ action: 'research_product' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /PERSONAL_SHOPPER_QUERY_REQUIRED/);
  });

  it('ranks candidate options', async () => {
    const result = await adapter.execute(
      {
        action: 'rank_options',
        query: 'executive office chair',
        budget_cents: 70000,
        candidates: [
          { name: 'Ergo Chair A', price_cents: 62000 },
          { name: 'Ergo Chair B', price_cents: 78000 }
        ]
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'personal-shopper');
    assert.ok(Array.isArray(result.data?.ranked_candidates));
  });
});
