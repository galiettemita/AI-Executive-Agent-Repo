import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('camsnap adapter', () => {
  it('requires camera id', async () => {
    const result = await adapter.execute({ action: 'capture_frame' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /CAMSNAP_CAMERA_REQUIRED/);
  });

  it('returns capture metadata', async () => {
    const result = await adapter.execute({ action: 'capture_frame', camera_id: 'front-door' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'camsnap');
  });
});
