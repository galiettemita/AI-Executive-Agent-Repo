import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('parcel-package-tracking adapter', () => {
  it('rejects short tracking numbers', async () => {
    const result = await adapter.execute({ tracking_number: '123' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.equal(result.error?.code, 'VALIDATION_FAILED');
  });

  it('returns parcel shipment timeline', async () => {
    const result = await adapter.execute(
      { tracking_number: '1Z999AA10123456784', carrier: 'auto' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'parcel');
    assert.equal(result.data?.tracking_number, '1Z999AA10123456784');
  });
});
