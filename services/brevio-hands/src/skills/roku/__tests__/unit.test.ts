import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('roku adapter', () => {
  it('requires action fields for launch_app', async () => {
    const result = await adapter.execute({ action: 'launch_app', device_id: 'roku-1' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /ROKU_ACTION_FIELDS_REQUIRED/);
  });

  it('returns status payload', async () => {
    const result = await adapter.execute({ action: 'status' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'roku');
  });
});
