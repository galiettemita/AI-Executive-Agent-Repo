import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('news-aggregator unit', () => {
  it('returns feed items', async () => {
    const output = await runClient({});
    assert.equal(output.provider, 'news-aggregator');
    assert.ok(output.items.length > 0);
  });

  it('filters by topic', async () => {
    const output = await runClient({ topic: 'github' });
    assert.ok(output.items.every((item) => `${item.source} ${item.title}`.toLowerCase().includes('github')));
  });
});
