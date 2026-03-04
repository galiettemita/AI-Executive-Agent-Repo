import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('samsung-smart-tv adapter', () => {
  it('requires app_id for launch_app action', async () => {
    const result = await adapter.execute({ action: 'launch_app' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /SAMSUNG_SMART_TV_APP_REQUIRED/);
  });

  it('returns deterministic status payload', async () => {
    const result = await adapter.execute({ action: 'status' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'samsung-smart-tv');
  });
});
