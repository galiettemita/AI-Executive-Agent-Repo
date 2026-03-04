import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('mole-mac-cleanup adapter', () => {
  it('requires confirmation for run_cleanup', async () => {
    const result = await adapter.execute({ action: 'run_cleanup' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /MOLE_MAC_CLEANUP_CONFIRMATION_REQUIRED/);
  });

  it('returns scan metrics', async () => {
    const result = await adapter.execute({ action: 'scan_cleanup', mode: 'quick' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'mole-mac-cleanup');
  });
});
