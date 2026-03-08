import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('jira unit', () => {
  it('lists issues', async () => {
    const output = await runClient({ action: 'issue_list', project_key: 'OPS' });
    assert.equal(output.provider, 'jira');
    assert.ok((output.issues?.length ?? 0) > 0);
  });

  it('creates issue', async () => {
    const output = await runClient({
      action: 'issue_create',
      project_key: 'OPS',
      summary: 'Add failover dashboard'
    });

    assert.equal(output.action, 'issue_create');
    assert.ok(output.issue_key?.startsWith('OPS-'));
  });
});
