import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('outlook unit', () => {
  it('lists inbox messages', async () => {
    const output = await runClient({ action: 'inbox_list' });
    assert.equal(output.provider, 'outlook');
    assert.ok((output.mails?.length ?? 0) > 0);
  });

  it('requires confirmation to send', async () => {
    const output = await runClient({
      action: 'send',
      to: ['ops@example.com'],
      subject: 'Status',
      body: 'All systems green'
    });

    assert.equal(output.action, 'send');
    assert.equal(output.confirmation_required, true);
  });
});
