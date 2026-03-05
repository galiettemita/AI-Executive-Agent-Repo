import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('spotify-history adapter', () => {
  it('returns listening summary', async () => {
    const result = await adapter.execute({ action: 'listening_summary', window: '4w' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'spotify-history');
    assert.equal(typeof result.data?.total_listening_minutes, 'number');
  });

  it('supports bounded top tracks output', async () => {
    const result = await adapter.execute({ action: 'top_tracks', limit: 1 }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal((result.data?.top_tracks ?? []).length, 1);
  });
});
