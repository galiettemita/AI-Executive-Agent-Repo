import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('react-email-skills adapter', () => {
  it('requires template id', async () => {
    const result = await adapter.execute({ action: 'render_template' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /REACT_EMAIL_TEMPLATE_REQUIRED/);
  });

  it('returns deterministic html output', async () => {
    const result = await adapter.execute({ action: 'render_template', template_id: 'welcome' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'react-email-skills');
  });
});
