import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('google-calendar unit', () => {
  it('requires confirmation for create', async () => {
    const output = await runClient({
      action: 'create',
      calendar_id: 'primary',
      event: {
        title: 'Dinner',
        start_time: '2026-03-05T19:00:00.000Z',
        end_time: '2026-03-05T20:00:00.000Z'
      }
    });

    assert.equal(output.confirmation_required, true);
    assert.equal(output.event_id, undefined);
  });

  it('returns events for list', async () => {
    const output = await runClient({ action: 'list', calendar_id: 'primary' });
    assert.equal(output.action, 'list');
    assert.ok((output.events?.length ?? 0) > 0);
  });
});
