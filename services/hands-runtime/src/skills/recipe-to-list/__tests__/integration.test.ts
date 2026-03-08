import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  provider: string;
  action: string;
  normalized_items_count: number;
  first_section: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'parse-success.json'), 'utf8')) as Fixture;

describe('recipe-to-list integration', () => {
  it('returns deterministic parsed ingredient list', async () => {
    const result = await adapter.execute(
      {
        action: 'parse_recipe',
        recipe_title: 'Weeknight bowl',
        recipe_text: '2 chicken\n1 tomato\npasta'
      },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, expected.provider);
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.normalized_items?.length, expected.normalized_items_count);
    assert.equal(result.data?.normalized_items?.[0]?.section, expected.first_section);
  });
});
