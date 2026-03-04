import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('pet-care adapter', () => {
  it('requires confirmation for booking visit', async () => {
    const result = await adapter.execute({ action: 'book_visit', provider_id: 'pet_001' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /PET_CARE_CONFIRMATION_REQUIRED/);
  });

  it('returns provider list', async () => {
    const result = await adapter.execute(
      { action: 'providers', pet_type: 'dog', service_type: 'daycare' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.ok(Array.isArray(result.data?.providers));
  });
});
