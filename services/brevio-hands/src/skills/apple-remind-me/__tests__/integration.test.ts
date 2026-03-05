import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  action: string;
  reminders_count: number;
  summary_contains: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'list-success.json'), 'utf8')) as Fixture;

describe('apple-remind-me integration', () => {
  it('returns deterministic reminder-list payload', async () => {
    const result = await adapter.execute({ action: 'list' }, {} as never);
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.reminders?.length, expected.reminders_count);
    assert.match(result.data?.summary ?? '', new RegExp(expected.summary_contains));
  });
});
