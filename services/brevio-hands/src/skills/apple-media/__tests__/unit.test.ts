import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('apple-media adapter', () => {
  it('requires device for playback status', async () => {
    const result = await adapter.execute({ action: 'playback_status' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /APPLE_MEDIA_DEVICE_REQUIRED/);
  });

  it('discovers local media devices', async () => {
    const result = await adapter.execute({ action: 'discover_devices' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'apple-media');
    assert.ok(Array.isArray(result.data?.devices));
  });
});
