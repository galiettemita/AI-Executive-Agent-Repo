import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('relationship-skills adapter', () => {
  it('requires context and goal', async () => {
    const result = await adapter.execute({ action: 'coach_message' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /RELATIONSHIP_SKILLS_CONTEXT_REQUIRED/);
  });

  it('returns communication guidance', async () => {
    const result = await adapter.execute(
      { action: 'coach_message', context: 'Recurring disagreement about household chores.', goal: 'agree on ownership' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'relationship-skills');
  });
});
