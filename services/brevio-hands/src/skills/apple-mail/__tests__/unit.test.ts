import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('apple-mail adapter', () => {
  it('requires confirmation for send', async () => {
    const result = await adapter.execute(
      {
        action: 'send',
        to: ['alex@example.com'],
        subject: 'Draft',
        body: 'Testing message',
        confirmed: false
      },
      {} as never
    );
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /APPLE_MAIL_CONFIRMATION_REQUIRED/);
  });

  it('returns inbox entries', async () => {
    const result = await adapter.execute({ action: 'list_inbox' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'apple-mail-local');
    assert.ok(Array.isArray(result.data?.emails));
  });
});
