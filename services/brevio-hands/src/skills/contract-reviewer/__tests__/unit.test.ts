import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('contract-reviewer adapter', () => {
  it('requires contract text', async () => {
    const result = await adapter.execute({ action: 'review_contract' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /CONTRACT_REVIEWER_TEXT_REQUIRED/);
  });

  it('returns risk summary', async () => {
    const result = await adapter.execute(
      {
        action: 'review_contract',
        contract_text:
          'This agreement includes indemnification, limitation of liability, and termination clauses requiring review before execution.'
      },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'contract-reviewer');
  });
});
