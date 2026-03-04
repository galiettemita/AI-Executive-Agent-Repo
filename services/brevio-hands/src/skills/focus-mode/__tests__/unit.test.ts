import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('focus-mode adapter', () => {
  it('requires start session fields', async () => {
    const result = await adapter.execute({ action: 'start_session' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /FOCUS_MODE_START_FIELDS_REQUIRED/);
  });

  it('starts deterministic focus session', async () => {
    const result = await adapter.execute(
      {
        action: 'start_session',
        goal: 'Finalize investor update',
        duration_minutes: 75
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'focus-mode');
    assert.ok(Array.isArray(result.data?.check_in_schedule));
  });
});
