import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  action: string;
  todos_count: number;
  inbox_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'list-today-success.json'), 'utf8')) as Fixture;

describe('things-mac integration', () => {
  it('returns deterministic today-list payload', async () => {
    const result = await adapter.execute({ action: 'list_today' }, {} as never);
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.todos?.length, expected.todos_count);
    assert.equal(result.data?.inbox_count, expected.inbox_count);
  });
});
