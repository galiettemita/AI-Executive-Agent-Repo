import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('just-fucking-cancel adapter', () => {
  it('requires input for scan action', async () => {
    const result = await adapter.execute({ action: 'scan_subscriptions' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /JUST_FUCKING_CANCEL_INPUT_REQUIRED/);
  });

  it('returns findings payload', async () => {
    const result = await adapter.execute(
      { action: 'scan_subscriptions', transactions_csv: 'date,merchant,amount\n2026-03-01,Streaming Service Pro,19.99' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'just-fucking-cancel');
  });
});
