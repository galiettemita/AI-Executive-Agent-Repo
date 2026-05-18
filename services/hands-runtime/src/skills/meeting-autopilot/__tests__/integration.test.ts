import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  decisions_count: number;
  action_items_count: number;
  first_owner: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'follow-up-success.json'), 'utf8')) as Fixture;
}

describe('meeting-autopilot integration', () => {
  it('returns deterministic meeting extraction output', async () => {
    const result = await adapter.execute(
      {
        action: 'draft_follow_up',
        meeting_title: 'Weekly Staff',
        transcript:
          'We decided to launch the pilot next week. TODO: Send launch checklist to all teams. Next we reviewed dependencies.',
        participants: ['Avery', 'Jordan']
      },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.decisions?.length, expected.decisions_count);
    assert.equal(result.data?.action_items?.length, expected.action_items_count);
    assert.equal(result.data?.action_items?.[0]?.owner, expected.first_owner);
    assert.match(result.data?.follow_up_email ?? '', /Subject: Weekly Staff follow-up/);
  });
});
