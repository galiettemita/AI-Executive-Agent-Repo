import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('refund-radar adapter', () => {
  it('requires merchant and amount for draft', async () => {
    const result = await adapter.execute({ action: 'draft_refund_request', merchant: 'StreamPlus' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /REFUND_RADAR_DRAFT_FIELDS_REQUIRED/);
  });

  it('returns recurring charge flags', async () => {
    const result = await adapter.execute({ action: 'scan_recurring_charges' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'refund-radar');
    assert.ok(Array.isArray(result.data?.flagged_charges));
  });
});
