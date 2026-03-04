import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('de-ai-ify adapter', () => {
  it('requires text', async () => {
    const result = await adapter.execute({ action: 'rewrite_text' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /DE_AI_IFY_TEXT_REQUIRED/);
  });

  it('returns rewritten text', async () => {
    const result = await adapter.execute({ action: 'rewrite_text', text: 'Moreover, we utilize this framework daily.' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'de-ai-ify');
  });
});
