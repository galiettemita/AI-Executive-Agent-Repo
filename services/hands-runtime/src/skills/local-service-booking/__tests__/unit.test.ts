import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('local-service-booking adapter', () => {
  it('requires confirmation for service booking', async () => {
    const result = await adapter.execute(
      { action: 'book_service', provider_id: 'svc_001' },
      {} as never
    );
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /LOCAL_SERVICE_BOOKING_CONFIRMATION_REQUIRED/);
  });

  it('returns provider search results', async () => {
    const result = await adapter.execute(
      { action: 'search_providers', service_type: 'handyman', zip_code: '78701' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.ok(Array.isArray(result.data?.providers));
  });
});
