import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('smtp-send unit', () => {
  it('requires confirmation before sending', async () => {
    const pending = await runClient({
      to: ['owner@example.com'],
      subject: 'Test subject',
      body: 'Test body'
    });

    assert.equal(pending.sent, false);
    assert.equal(pending.confirmation_required, true);

    const sent = await runClient({
      to: ['owner@example.com'],
      subject: 'Test subject',
      body: 'Test body',
      confirmed: true
    });

    assert.equal(sent.sent, true);
    assert.equal(sent.confirmation_required, false);
  });
});
