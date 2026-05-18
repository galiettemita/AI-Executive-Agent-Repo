import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('alter-actions adapter', () => {
  it('requires confirmation for trigger action', async () => {
    const result = await adapter.execute(
      {
        action: 'trigger_action',
        action_key: 'things.new_todo'
      },
      {} as never
    );

    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /ALTER_ACTIONS_CONFIRMATION_REQUIRED/);
  });

  it('lists available callback actions', async () => {
    const result = await adapter.execute({ action: 'list_actions' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'alter-actions');
    assert.ok(Array.isArray(result.data?.actions));
  });
});
