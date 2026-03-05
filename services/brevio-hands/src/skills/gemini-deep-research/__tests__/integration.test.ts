import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  report_sections_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'research-success.json'), 'utf8')) as Fixture;

describe('gemini-deep-research integration', () => {
  it('returns deterministic deep research payload', async () => {
    const result = await adapter.execute(
      { action: 'run_research', topic: 'competitor intelligence', depth: 'deep' },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.report_sections?.length, expected.report_sections_count);
  });
});
