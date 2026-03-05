import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  current_mode: string;
  schedule_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'schedule-success.json'), 'utf8')) as Fixture;

describe('get-focus-mode integration', () => {
  it('returns deterministic focus schedule payload', async () => {
    const result = await adapter.execute({ action: 'upcoming_schedule' }, {} as never);
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.current_mode, expected.current_mode);
    assert.equal(result.data?.schedule?.length, expected.schedule_count);
  });
});
