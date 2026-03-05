import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('bill-pay-p2p adapter', () => {
  it('requires confirmation for payment creation', async () => {
    const result = await adapter.execute(
      { action: 'create_payment', payee_id: 'payee_001', amount_cents: 3500 },
      {} as never
    );
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /BILL_PAY_CONFIRMATION_REQUIRED/);
  });

  it('returns payee list', async () => {
    const result = await adapter.execute({ action: 'list_payees' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'bill-pay-p2p');
    assert.ok(Array.isArray(result.data?.payees));
  });
});
