import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  action: string;
  messages_count: number;
  mailbox: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'search-success.json'), 'utf8')) as Fixture;
}

describe('imap-email integration', () => {
  it('returns deterministic mailbox search payload', async () => {
    const result = await adapter.execute({ action: 'search', query: 'invoice' }, {} as never);

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.messages?.length, expected.messages_count);
    assert.equal(result.data?.mailbox, expected.mailbox);
  });
});
