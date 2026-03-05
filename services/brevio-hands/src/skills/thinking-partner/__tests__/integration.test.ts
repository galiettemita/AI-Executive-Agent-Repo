import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  questions_count: number;
  assumptions_count: number;
  matrix_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'decision-matrix-success.json'), 'utf8')) as Fixture;
}

describe('thinking-partner integration', () => {
  it('returns deterministic decision matrix payload', async () => {
    const result = await adapter.execute(
      {
        action: 'decision_matrix',
        topic: 'Choose launch strategy',
        constraints: ['Limited marketing budget'],
        options: ['Option A', 'Option B']
      },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.questions?.length, expected.questions_count);
    assert.equal(result.data?.assumptions_to_test?.length, expected.assumptions_count);
    assert.equal(result.data?.decision_matrix?.length, expected.matrix_count);
  });
});
