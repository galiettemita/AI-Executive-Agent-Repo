import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('track17 adapter', () => {
  it('rejects invalid tracking number length', async () => {
    const result = await adapter.execute({ tracking_number: 'abc' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.equal(result.error?.code, 'VALIDATION_FAILED');
  });

  it('returns 17track checkpoints', async () => {
    const result = await adapter.execute({ tracking_number: 'YT1234567890CN' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, '17track');
    assert.equal(result.data?.status, 'in_transit');
  });
});
