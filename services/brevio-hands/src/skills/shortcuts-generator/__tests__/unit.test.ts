import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('shortcuts-generator adapter', () => {
  it('requires name and steps when generating shortcut', async () => {
    const result = await adapter.execute({ action: 'generate_shortcut', shortcut_name: 'Morning flow' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /SHORTCUTS_GENERATOR_STEPS_REQUIRED/);
  });

  it('returns deterministic list payload', async () => {
    const result = await adapter.execute({ action: 'list_shortcuts' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'shortcuts-generator');
    assert.equal(Array.isArray(result.data?.shortcuts), true);
  });
});
