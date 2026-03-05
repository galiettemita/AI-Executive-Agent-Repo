import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('slack adapter', () => {
  it('validates required post fields', async () => {
    const result = await adapter.execute({ action: 'post_message', channel_id: 'C123' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.equal(result.error?.code, 'VALIDATION_FAILED');
    assert.match(result.error?.message ?? '', /SLACK_POST_FIELDS_REQUIRED/);
  });

  it('lists channels', async () => {
    const result = await adapter.execute({ action: 'list_channels' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'slack');
    assert.ok(Array.isArray(result.data?.channels));
  });
});
