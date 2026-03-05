import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  pages_processed: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'extract-success.json'), 'utf8')) as Fixture;

describe('pdf-tools integration', () => {
  it('returns deterministic text-extraction payload', async () => {
    const result = await adapter.execute(
      {
        action: 'extract_text',
        files: ['briefing.pdf'],
        output_name: 'briefing-summary.pdf'
      },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.pages_processed, expected.pages_processed);
  });
});
