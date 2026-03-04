import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('proactive-research adapter', () => {
  it('requires topic', async () => {
    const result = await adapter.execute({ action: 'monitor_topic' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /PROACTIVE_RESEARCH_TOPIC_REQUIRED/);
  });

  it('returns deterministic alerts', async () => {
    const result = await adapter.execute({ action: 'monitor_topic', topic: 'AI regulation' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'proactive-research');
  });
});
