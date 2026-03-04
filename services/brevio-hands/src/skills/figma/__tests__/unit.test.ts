import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('figma adapter', () => {
  it('requires file_key for every action', async () => {
    const result = await adapter.execute({ action: 'analyze_file' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /FIGMA_FILE_KEY_REQUIRED/);
  });

  it('returns deterministic findings payload', async () => {
    const result = await adapter.execute({ action: 'analyze_file', file_key: 'abc123' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'figma');
  });
});
