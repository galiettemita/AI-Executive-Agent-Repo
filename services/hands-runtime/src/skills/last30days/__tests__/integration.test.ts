import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  highlights_count: number;
  sources_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'scan-success.json'), 'utf8')) as Fixture;

describe('last30days integration', () => {
  it('returns deterministic recent trend scan payload', async () => {
    const result = await adapter.execute({ action: 'scan_topic', query: 'ai assistants' }, {} as never);
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.highlights?.length, expected.highlights_count);
    assert.equal(result.data?.sources?.length, expected.sources_count);
  });
});
