import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  sent: boolean;
  confirmation_required: boolean;
  recipients_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'send-success.json'), 'utf8')) as Fixture;
}

describe('smtp-send integration', () => {
  it('returns deterministic delivery confirmation payload', async () => {
    const result = await adapter.execute(
      {
        to: ['exec@example.com'],
        subject: 'Board packet ready',
        body: 'The board packet is ready for review.',
        confirmed: true
      },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.sent, expected.sent);
    assert.equal(result.data?.confirmation_required, expected.confirmation_required);
    assert.equal(result.data?.recipients?.length, expected.recipients_count);
    assert.match(result.data?.message_id ?? '', /^msg_[a-f0-9]{16}$/);
  });
});
