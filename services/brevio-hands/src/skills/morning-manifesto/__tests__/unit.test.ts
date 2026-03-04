import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('morning-manifesto adapter', () => {
  it('requires goals for manifesto generation', async () => {
    const result = await adapter.execute({ action: 'generate_manifesto' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /MORNING_MANIFESTO_GOALS_REQUIRED/);
  });

  it('returns manifesto and actionable items', async () => {
    const result = await adapter.execute(
      {
        action: 'generate_manifesto',
        goals: ['Finish Q2 hiring plan', 'Finalize investor memo'],
        gratitude: ['Strong team support'],
        tone: 'supportive'
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'morning-manifesto');
    assert.ok(Array.isArray(result.data?.action_items));
  });
});
