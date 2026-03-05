import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('pros-cons adapter', () => {
  it('requires decision and options', async () => {
    const result = await adapter.execute({ action: 'evaluate_decision' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /PROS_CONS_DECISION_FIELDS_REQUIRED/);
  });

  it('returns deterministic recommendation', async () => {
    const result = await adapter.execute(
      { action: 'evaluate_decision', decision: 'Choose office location', options: ['Downtown', 'Suburb'] },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'pros-cons');
  });
});
