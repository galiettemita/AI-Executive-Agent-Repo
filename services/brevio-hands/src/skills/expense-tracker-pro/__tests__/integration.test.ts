import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  entries_count: number;
  total_cents: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'add-expense-success.json'), 'utf8')) as Fixture;

describe('expense-tracker-pro integration', () => {
  it('returns deterministic expense aggregation payload', async () => {
    const result = await adapter.execute(
      {
        action: 'add_expense',
        merchant: 'Airport taxi',
        amount_cents: 5000,
        category: 'Transport',
        occurred_on: '2026-03-04'
      },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.entries?.length, expected.entries_count);
    assert.equal(result.data?.total_cents, expected.total_cents);
  });
});
