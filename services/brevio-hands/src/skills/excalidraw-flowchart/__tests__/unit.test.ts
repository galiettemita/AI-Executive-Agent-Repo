import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('excalidraw-flowchart adapter', () => {
  it('requires description for generate_flowchart', async () => {
    const result = await adapter.execute({ action: 'generate_flowchart' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /EXCALIDRAW_DESCRIPTION_REQUIRED/);
  });

  it('returns deterministic graph payload', async () => {
    const result = await adapter.execute({ action: 'generate_flowchart', description: 'Build release process' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'excalidraw-flowchart');
  });
});
