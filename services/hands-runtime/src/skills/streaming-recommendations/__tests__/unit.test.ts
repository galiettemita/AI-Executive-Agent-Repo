import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('streaming-recommendations adapter', () => {
  it('requires confirmation for watchlist add', async () => {
    const result = await adapter.execute(
      { action: 'watchlist_add', title: 'The Executive Signal' },
      {} as never
    );
    assert.equal(result.status, 'FAILED');
    assert.match(
      result.error?.message ?? '',
      /STREAMING_RECOMMENDATIONS_CONFIRMATION_REQUIRED/
    );
  });

  it('returns recommendation picks', async () => {
    const result = await adapter.execute({ action: 'recommend', mood: 'focused' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.ok(Array.isArray(result.data?.recommendations));
  });
});
