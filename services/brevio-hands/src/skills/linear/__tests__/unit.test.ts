import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('linear unit', () => {
  it('lists issues', async () => {
    const output = await runClient({ action: 'issue_list', team_id: 'ENG' });
    assert.equal(output.provider, 'linear');
    assert.ok((output.issues?.length ?? 0) > 0);
  });

  it('creates issue', async () => {
    const output = await runClient({
      action: 'issue_create',
      team_id: 'ENG',
      title: 'Ship workflow replay dashboard'
    });

    assert.equal(output.action, 'issue_create');
    assert.ok(output.issue_id?.startsWith('lin_'));
  });
});
