import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('sonoscli adapter', () => {
  it('requires speaker/query for play action', async () => {
    const result = await adapter.execute({ action: 'play', speaker_id: 'sonos-living-room' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /SONOSCLI_PLAY_FIELDS_REQUIRED/);
  });

  it('returns deterministic zone discovery', async () => {
    const result = await adapter.execute({ action: 'discover' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'sonoscli');
    assert.equal(Array.isArray(result.data?.zones), true);
  });
});
