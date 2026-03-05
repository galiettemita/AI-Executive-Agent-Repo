import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  findings_count: number;
  draft_prefix: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'draft-success.json'), 'utf8')) as Fixture;

describe('just-fucking-cancel integration', () => {
  it('returns deterministic cancellation-draft payload', async () => {
    const result = await adapter.execute(
      {
        action: 'draft_cancellation',
        merchant_name: 'Gym Membership'
      },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.findings?.length, expected.findings_count);
    assert.equal((result.data?.draft_message as string).startsWith(expected.draft_prefix), true);
  });
});
