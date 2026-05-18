import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('post-at adapter', () => {
  it('requires tracking number', async () => {
    const result = await adapter.execute({ action: 'track_parcel' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /POST_AT_TRACKING_REQUIRED/);
  });

  it('returns checkpoint history', async () => {
    const result = await adapter.execute({ action: 'track_parcel', tracking_number: 'AT123456789' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'post-at');
  });
});
