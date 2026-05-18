import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  action: string;
  cards_count: number;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'card-list-success.json'), 'utf8')) as Fixture;

describe('trello integration', () => {
  it('returns deterministic card-list payload', async () => {
    const result = await adapter.execute(
      { action: 'card_list', board_id: 'exec_board' },
      {} as never
    );
    const expected = readFixture();

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.action, expected.action);
    assert.equal(result.data?.cards?.length, expected.cards_count);
  });
});
