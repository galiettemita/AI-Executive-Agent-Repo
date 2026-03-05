import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('healthkit-sync adapter', () => {
  it('requires range or days', async () => {
    const result = await adapter.execute({ action: 'sync_all' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /HEALTHKIT_SYNC_ALIAS_RANGE_REQUIRED/);
  });

  it('returns alias forwarding metadata', async () => {
    const result = await adapter.execute(
      {
        action: 'sync_sleep',
        days: 3
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'healthkit-sync');
    assert.equal(result.data?.alias_target, 'healthkit-sync-apple');
  });
});
