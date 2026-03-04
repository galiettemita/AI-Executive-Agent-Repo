import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('apple-contacts unit', () => {
  it('finds matching contacts by name', async () => {
    const output = await runClient({ query: 'sarah' });
    assert.equal(output.provider, 'apple-contacts-local');
    assert.ok((output.contacts.length ?? 0) > 0);
  });

  it('returns empty list for unknown contact', async () => {
    const output = await runClient({ query: 'no-such-contact' });
    assert.equal(output.contacts.length, 0);
  });
});
