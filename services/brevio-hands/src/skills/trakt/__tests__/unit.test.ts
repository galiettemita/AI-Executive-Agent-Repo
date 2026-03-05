import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('trakt adapter', () => {
  it('requires media_id when marking watched', async () => {
    const result = await adapter.execute({ action: 'mark_watched' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /TRAKT_MEDIA_ID_REQUIRED/);
  });

  it('returns trending items', async () => {
    const result = await adapter.execute({ action: 'trending' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'trakt');
    assert.ok(Array.isArray(result.data?.items));
  });
});
