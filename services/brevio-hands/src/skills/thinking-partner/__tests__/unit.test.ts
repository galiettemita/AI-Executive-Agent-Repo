import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('thinking-partner adapter', () => {
  it('requires topic', async () => {
    const result = await adapter.execute({ action: 'clarify_problem' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /THINKING_PARTNER_TOPIC_REQUIRED/);
  });

  it('returns decision matrix for options', async () => {
    const result = await adapter.execute(
      {
        action: 'decision_matrix',
        topic: 'Choose roadmap sequencing',
        options: ['Ship integrations first', 'Ship mobile first'],
        constraints: ['Team capacity fixed']
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'thinking-partner');
    assert.ok(Array.isArray(result.data?.decision_matrix));
  });
});
