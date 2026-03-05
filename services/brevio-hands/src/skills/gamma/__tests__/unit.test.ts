import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('gamma adapter', () => {
  it('requires topic for create_deck', async () => {
    const result = await adapter.execute({ action: 'create_deck' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /GAMMA_TOPIC_REQUIRED/);
  });

  it('returns deterministic deck payload', async () => {
    const result = await adapter.execute({ action: 'create_deck', topic: 'Quarterly strategy' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'gamma');
  });
});
