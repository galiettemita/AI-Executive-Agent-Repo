import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('granola adapter', () => {
  it('requires note text', async () => {
    const result = await adapter.execute({ action: 'summarize_note' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /GRANOLA_NOTE_TEXT_REQUIRED/);
  });

  it('returns deterministic action items', async () => {
    const result = await adapter.execute(
      { action: 'extract_actions', note_text: 'Team agreed to revise roadmap and schedule architecture review.' },
      {} as never
    );
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'granola');
  });
});
