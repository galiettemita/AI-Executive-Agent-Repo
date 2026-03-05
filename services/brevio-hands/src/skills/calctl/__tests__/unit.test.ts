import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('calctl adapter', () => {
  it('requires title/start/end when creating events', async () => {
    const result = await adapter.execute({ action: 'create_event', title: 'Demo' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /CALCTL_EVENT_FIELDS_REQUIRED/);
  });

  it('lists deterministic events', async () => {
    const result = await adapter.execute({ action: 'list_events' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'apple-calendar');
    assert.equal(Array.isArray(result.data?.events), true);
  });
});
