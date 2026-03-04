import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  flagged_count: number;
  summary: string;
  draft_contains: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'draft-success.json'), 'utf8')) as Fixture;
}

describe('refund-radar integration', () => {
  it('returns deterministic refund-draft payload', async () => {
    const result = await adapter.execute(
      {
        action: 'draft_refund_request',
        merchant: 'StreamPlus',
        amount_cents: 1599,
        reason: 'I did not intend to renew this subscription.'
      },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.flagged_charges?.length, expected.flagged_count);
    assert.equal(result.data?.summary, expected.summary);
    assert.match(result.data?.draft_message ?? '', new RegExp(expected.draft_contains));
  });
});
