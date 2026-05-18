import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('pharmacy-prescription adapter', () => {
  it('requires confirmation for refill request', async () => {
    const result = await adapter.execute(
      {
        action: 'refill_request',
        prescription_id: 'rx_001',
        pharmacy_name: 'Downtown Pharmacy'
      },
      {} as never
    );
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /PHARMACY_REFILL_CONFIRMATION_REQUIRED/);
  });

  it('returns medication lookup options', async () => {
    const result = await adapter.execute(
      { action: 'medication_lookup', medication_name: 'Atorvastatin' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.ok(Array.isArray(result.data?.medications));
  });
});
