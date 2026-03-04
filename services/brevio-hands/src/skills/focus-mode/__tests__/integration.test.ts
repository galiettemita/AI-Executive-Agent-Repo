import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  session_id: string;
  status: string;
  schedule_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'start-success.json'), 'utf8')) as Fixture;
}

describe('focus-mode integration', () => {
  it('returns deterministic focus session payload', async () => {
    const result = await adapter.execute(
      { action: 'start_session', goal: 'Draft investor memo', duration_minutes: 50 },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.session_id, expected.session_id);
    assert.equal(result.data?.status, expected.status);
    assert.equal(result.data?.check_in_schedule?.length, expected.schedule_count);
  });
});
