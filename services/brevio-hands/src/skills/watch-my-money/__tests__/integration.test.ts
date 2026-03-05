import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  alerts_count: number;
  spend_rate_pct_of_income: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'analyze-success.json'), 'utf8')) as Fixture;

describe('watch-my-money integration', () => {
  it('returns deterministic budget alert analysis payload', async () => {
    const result = await adapter.execute(
      {
        action: 'analyze_statement',
        monthly_income_cents: 40000,
        transactions: [
          { category: 'Dining', amount_cents: 25000 },
          { category: 'Transport', amount_cents: 5000 }
        ]
      },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.alerts?.length, expected.alerts_count);
    assert.equal(result.data?.spend_rate_pct_of_income, expected.spend_rate_pct_of_income);
  });
});
