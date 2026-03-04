import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('tax-professional adapter', () => {
  it('requires tax year', async () => {
    const result = await adapter.execute({ action: 'filing_checklist' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /TAX_PROFESSIONAL_TAX_YEAR_REQUIRED/);
  });

  it('returns checklist with disclaimer', async () => {
    const result = await adapter.execute(
      {
        action: 'filing_checklist',
        tax_year: 2025,
        filing_status: 'single'
      },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'tax-professional');
    assert.equal(result.data?.disclaimer, 'not_tax_advice');
  });
});
