import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  entries_count: number;
  today_spend_cents: number;
  month_spend_cents: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'log-expense-success.json'), 'utf8')) as Fixture;
}

describe('smart-expense-tracker integration', () => {
  it('returns deterministic spend summary', async () => {
    const result = await adapter.execute(
      {
        action: 'log_expense',
        merchant: 'Airport Coffee',
        amount_cents: 5000,
        category: 'Dining',
        occurred_on: '2026-03-04'
      },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.entries?.length, expected.entries_count);
    assert.equal(result.data?.today_spend_cents, expected.today_spend_cents);
    assert.equal(result.data?.month_spend_cents, expected.month_spend_cents);
  });
});
