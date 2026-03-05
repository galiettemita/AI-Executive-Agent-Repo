import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  quotes_count: number;
  first_symbol: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'quotes-success.json'), 'utf8')) as Fixture;

describe('yahoo-finance integration', () => {
  it('returns deterministic quote payload', async () => {
    const result = await adapter.execute(
      { action: 'quotes', symbols: ['AAPL', 'MSFT'] },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.quotes?.length, expected.quotes_count);
    assert.equal(result.data?.quotes?.[0]?.symbol, expected.first_symbol);
  });
});
