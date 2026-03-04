import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('get-focus-mode adapter', () => {
  it('returns current mode status', async () => {
    const result = await adapter.execute({ action: 'current_mode' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'get-focus-mode');
    assert.equal(typeof result.data?.current_mode, 'string');
  });

  it('returns upcoming schedule', async () => {
    const result = await adapter.execute({ action: 'upcoming_schedule' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.ok(Array.isArray(result.data?.schedule));
  });
});
