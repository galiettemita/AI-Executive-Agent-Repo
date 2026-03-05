import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('tmdb unit', () => {
  it('returns recommendations', async () => {
    const output = await runClient({});
    assert.equal(output.provider, 'tmdb');
    assert.ok(output.results.length > 0);
  });

  it('filters by query', async () => {
    const output = await runClient({ query: 'strategy' });
    assert.ok(output.results.every((item) => `${item.title} ${item.overview}`.toLowerCase().includes('strategy')));
  });
});
