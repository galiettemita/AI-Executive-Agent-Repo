import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  recommended_card: string;
  estimated_reward_cents: number;
  alternatives_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'recommend-success.json'), 'utf8')) as Fixture;
}

describe('card-optimizer integration', () => {
  it('returns deterministic card recommendation ranking', async () => {
    const result = await adapter.execute(
      { action: 'recommend_card', purchase_category: 'groceries', amount_cents: 10000 },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.recommended_card, expected.recommended_card);
    assert.equal(result.data?.estimated_reward_cents, expected.estimated_reward_cents);
    assert.equal(result.data?.alternatives?.length, expected.alternatives_count);
  });
});
