import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('card-optimizer adapter', () => {
  it('requires purchase category', async () => {
    const result = await adapter.execute({ action: 'recommend_card', amount_cents: 12000 }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /CARD_OPTIMIZER_CATEGORY_REQUIRED/);
  });

  it('returns recommendation and alternatives', async () => {
    const result = await adapter.execute(
      { action: 'recommend_card', purchase_category: 'groceries', amount_cents: 12500 },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'card-optimizer');
    assert.equal(typeof result.data?.estimated_reward_cents, 'number');
  });
});
