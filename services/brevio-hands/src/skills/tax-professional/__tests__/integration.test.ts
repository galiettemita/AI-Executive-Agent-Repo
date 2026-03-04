import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  estimated_deductions_cents: number;
  checklist_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'estimate-success.json'), 'utf8')) as Fixture;

describe('tax-professional integration', () => {
  it('returns deterministic tax-planning payload', async () => {
    const result = await adapter.execute(
      {
        action: 'estimate_deductions',
        tax_year: 2026,
        filing_status: 'single',
        deductible_expenses_cents: [
          { category: 'Home office', amount_cents: 15000 },
          { category: 'Education', amount_cents: 5000 }
        ]
      },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.estimated_deductions_cents, expected.estimated_deductions_cents);
    assert.equal(result.data?.checklist?.length, expected.checklist_count);
  });
});
