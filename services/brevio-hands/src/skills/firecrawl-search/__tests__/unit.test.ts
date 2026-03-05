import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('firecrawl-search unit', () => {
  it('returns crawl results', async () => {
    const output = await runClient({ query: 'executive digest' });
    assert.equal(output.provider, 'firecrawl');
    assert.ok(output.results.length > 0);
  });
});
