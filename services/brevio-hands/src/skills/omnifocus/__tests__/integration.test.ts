import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  action: string;
  tasks_count: number;
  flagged_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'list-flagged-success.json'), 'utf8')) as Fixture;

describe('omnifocus integration', () => {
  it('returns deterministic flagged-task payload', async () => {
    const result = await adapter.execute({ action: 'list_flagged' }, {} as never);
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.tasks?.length, expected.tasks_count);
    assert.equal(result.data?.flagged_count, expected.flagged_count);
  });
});
