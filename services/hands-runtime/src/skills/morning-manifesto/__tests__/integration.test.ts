import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  action_items_count: number;
  affirmations_count: number;
  manifesto_contains: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'manifesto-success.json'), 'utf8')) as Fixture;
}

describe('morning-manifesto integration', () => {
  it('returns deterministic manifesto structure', async () => {
    const result = await adapter.execute(
      {
        action: 'generate_manifesto',
        goals: ['Finalize investor update', 'Ship roadmap memo'],
        blockers: ['Pending legal review'],
        gratitude: ['Team support'],
        tone: 'direct'
      },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.action_items?.length, expected.action_items_count);
    assert.equal(result.data?.affirmations?.length, expected.affirmations_count);
    assert.match(result.data?.manifesto ?? '', new RegExp(expected.manifesto_contains));
  });
});
