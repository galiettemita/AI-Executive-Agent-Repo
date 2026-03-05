import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  action: string;
  calendar_id: string;
  events_count: number;
  confirmation_required: boolean;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'list-success.json'), 'utf8')) as Fixture;

describe('google-calendar integration', () => {
  it('returns deterministic calendar listing payload', async () => {
    const result = await adapter.execute({ action: 'list' }, {} as never);
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.calendar_id, expected.calendar_id);
    assert.equal(result.data?.events?.length, expected.events_count);
    assert.equal(result.data?.confirmation_required, expected.confirmation_required);
  });
});
