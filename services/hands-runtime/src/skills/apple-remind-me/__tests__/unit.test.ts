import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('apple-remind-me adapter', () => {
  it('requires title when creating reminder', async () => {
    const result = await adapter.execute({ action: 'create' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /APPLE_REMIND_ME_TITLE_REQUIRED/);
  });

  it('creates reminder deterministically', async () => {
    const result = await adapter.execute({ action: 'create', title: 'Pay utilities' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'apple-reminders');
    assert.equal(result.data?.reminders?.[0]?.title, 'Pay utilities');
  });
});
