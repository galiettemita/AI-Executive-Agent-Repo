import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('brave-search unit', () => {
  it('returns matched results', async () => {
    const output = await runClient({ query: 'context budgets' });
    assert.equal(output.provider, 'brave-search');
    assert.ok(output.results.length > 0);
  });
});
