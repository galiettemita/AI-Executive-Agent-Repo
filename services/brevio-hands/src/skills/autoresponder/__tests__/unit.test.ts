import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('autoresponder adapter', () => {
  it('requires intercepted text for intercept action', async () => {
    const result = await adapter.execute({ action: 'intercept' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /AUTORESPONDER_INTERCEPT_TEXT_REQUIRED/);
  });

  it('returns delegated response metadata', async () => {
    const result = await adapter.execute(
      {
        action: 'intercept',
        incoming_text: 'Can you call me back?',
        channel: 'imessage',
        delegation_enabled: true
      },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'autoresponder');
    assert.equal(result.data?.delegated_to_brain, true);
  });
});
