import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  overall_risk: string;
  risk_items_count: number;
  must_review_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'review-success.json'), 'utf8')) as Fixture;
}

describe('contract-reviewer integration', () => {
  it('returns deterministic contract risk summary', async () => {
    const result = await adapter.execute(
      {
        action: 'review_contract',
        contract_text:
          'This agreement is made between parties for services. Liability terms are broad and termination rights are unilateral with 7 days notice for convenience. This text exceeds minimum length for validation.',
        contract_type: 'msa',
        jurisdiction: 'US-NY'
      },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.overall_risk, expected.overall_risk);
    assert.equal(result.data?.risk_items?.length, expected.risk_items_count);
    assert.equal(result.data?.must_review_clauses?.length, expected.must_review_count);
  });
});
