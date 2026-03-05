import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  recommendations_count: number;
  first_urgency: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'recommendations-success.json'), 'utf8')) as Fixture;
}

describe('clawringhouse integration', () => {
  it('returns deterministic proactive recommendation set', async () => {
    const result = await adapter.execute(
      {
        action: 'proactive_recommendations',
        household_items: [
          {
            name: 'Coffee Beans',
            days_since_last_order: 33,
            typical_cycle_days: 30,
            estimated_units_left: 1
          },
          {
            name: 'Dish Soap',
            days_since_last_order: 5,
            typical_cycle_days: 20,
            estimated_units_left: 3
          }
        ]
      },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.recommendations?.length, expected.recommendations_count);
    assert.equal(result.data?.recommendations?.[0]?.urgency, expected.first_urgency);
  });
});
