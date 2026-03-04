import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('exa unit', () => {
  it('returns filtered results by query', async () => {
    const output = await runClient({ query: 'executive planning' });
    assert.equal(output.provider, 'exa');
    assert.ok(output.results.length > 0);
  });

  it('supports domain filtering', async () => {
    const output = await runClient({
      query: 'guide',
      include_domains: ['docs.example.com']
    });

    assert.ok(output.results.every((item) => item.url.includes('docs.example.com')));
  });
});
