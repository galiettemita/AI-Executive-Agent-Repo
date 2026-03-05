import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('recipe-to-list adapter', () => {
  it('requires recipe text for parse_recipe', async () => {
    const result = await adapter.execute({ action: 'parse_recipe' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /RECIPE_TO_LIST_TEXT_REQUIRED/);
  });

  it('syncs normalized items to todoist tasks', async () => {
    const result = await adapter.execute(
      {
        action: 'sync_todoist',
        recipe_title: 'Weeknight Pasta',
        recipe_items: [
          { item: 'pasta', quantity: '1 box' },
          { item: 'tomato sauce', quantity: '2 jars' }
        ],
        project_id: 'todoist_project_1'
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'recipe-to-list');
    assert.ok(Array.isArray(result.data?.task_ids));
  });
});
