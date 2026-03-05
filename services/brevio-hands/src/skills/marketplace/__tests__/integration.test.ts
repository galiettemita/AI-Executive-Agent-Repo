import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  fair_price_cents: number;
  scam_risk: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'evaluate-success.json'), 'utf8')) as Fixture;

describe('marketplace integration', () => {
  it('returns deterministic valuation output', async () => {
    const result = await adapter.execute(
      {
        action: 'evaluate_listing',
        title: 'Used road bike',
        condition: 'good',
        asking_price_cents: 13000,
        comparable_prices_cents: [10000, 12000, 14000]
      },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.fair_price_cents, expected.fair_price_cents);
    assert.equal(result.data?.scam_risk, expected.scam_risk);
  });
});
