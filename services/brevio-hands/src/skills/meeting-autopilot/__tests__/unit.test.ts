import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('meeting-autopilot adapter', () => {
  it('requires transcript', async () => {
    const result = await adapter.execute({ action: 'summarize_meeting' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /MEETING_AUTOPILOT_TRANSCRIPT_REQUIRED/);
  });

  it('creates summary and action items', async () => {
    const result = await adapter.execute(
      {
        action: 'extract_actions',
        transcript:
          'We decided to finalize pricing by Friday. TODO: Send revised pricing grid. TODO: Book legal review.',
        participants: ['Ari', 'Jordan']
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'meeting-autopilot');
    assert.ok(Array.isArray(result.data?.action_items));
  });
});
