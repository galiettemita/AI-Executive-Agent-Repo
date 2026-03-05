import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('google-workspace unit', () => {
  it('lists gmail messages', async () => {
    const output = await runClient({ action: 'gmail_list' });
    assert.equal(output.provider, 'google-workspace');
    assert.ok((output.mails?.length ?? 0) > 0);
  });

  it('requires confirmation to send gmail', async () => {
    const output = await runClient({
      action: 'gmail_send',
      to: ['team@example.com'],
      subject: 'Update',
      body: 'Weekly summary'
    });

    assert.equal(output.action, 'gmail_send');
    assert.equal(output.confirmation_required, true);
  });
});
