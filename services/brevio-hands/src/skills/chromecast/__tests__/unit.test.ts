import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('chromecast adapter', () => {
  it('requires device/media when casting', async () => {
    const result = await adapter.execute({ action: 'cast_media', device_name: 'Living Room Chromecast' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /CHROMECAST_CAST_FIELDS_REQUIRED/);
  });

  it('returns discovered devices', async () => {
    const result = await adapter.execute({ action: 'discover_devices' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'chromecast');
    assert.equal(Array.isArray(result.data?.devices), true);
  });
});
