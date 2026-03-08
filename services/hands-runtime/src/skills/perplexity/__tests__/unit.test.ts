import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('perplexity unit', () => {
  it('returns answer with citations', async () => {
    const output = await runClient({ query: 'executive operating model updates' });
    assert.equal(output.provider, 'perplexity');
    assert.ok(output.answer.length > 0);
    assert.ok(output.citations.length > 0);
  });
});
