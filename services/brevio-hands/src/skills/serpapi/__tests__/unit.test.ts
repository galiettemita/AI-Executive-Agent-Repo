import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('serpapi unit', () => {
  it('returns google results', async () => {
    const output = await runClient({ query: 'executive automation', engine: 'google' });
    assert.equal(output.provider, 'serpapi');
    assert.equal(output.engine, 'google');
  });

  it('filters to selected engine', async () => {
    const output = await runClient({ query: 'cafe', engine: 'yelp' });
    assert.equal(output.engine, 'yelp');
    assert.ok(output.results.every((item) => item.source === 'Yelp'));
  });
});
