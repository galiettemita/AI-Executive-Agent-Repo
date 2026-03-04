import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  score: number;
  recommendations_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'score-success.json'), 'utf8')) as Fixture;

describe('resume-builder integration', () => {
  it('returns deterministic resume scoring payload', async () => {
    const result = await adapter.execute(
      {
        action: 'score',
        role: 'Operations Lead',
        resume_markdown: '# Resume\n\nExperience in operations and systems.'
      },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.score, expected.score);
    assert.equal(result.data?.recommendations?.length, expected.recommendations_count);
  });
});
