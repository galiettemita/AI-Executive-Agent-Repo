import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('healthkit-sync-apple adapter', () => {
  it('requires range or days', async () => {
    const result = await adapter.execute({ action: 'sync_all' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /HEALTHKIT_SYNC_APPLE_RANGE_REQUIRED/);
  });

  it('syncs snapshots for requested metric', async () => {
    const result = await adapter.execute(
      {
        action: 'sync_steps',
        days: 7
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'healthkit-sync-apple');
    assert.ok(Array.isArray(result.data?.snapshots));
  });
});
