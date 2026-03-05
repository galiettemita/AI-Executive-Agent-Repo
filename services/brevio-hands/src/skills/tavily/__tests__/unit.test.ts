import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('tavily unit', () => {
  it('returns bounded result set', async () => {
    const output = await runClient({
      query: 'executive assistant ai',
      max_results: 3,
      include_domains: ['example.org']
    });

    assert.equal(output.provider, 'tavily');
    assert.equal(output.results.length, 3);
    assert.ok(output.results.every((item) => item.url.includes('example.org')));
  });
});
