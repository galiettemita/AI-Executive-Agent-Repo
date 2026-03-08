import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('fal-ai unit', () => {
  it('returns deterministic image output', async () => {
    const output = await runClient({
      prompt: 'Minimalist product hero image with blue accents',
      size: 'landscape'
    });

    assert.equal(output.provider, 'fal-ai');
    assert.ok(output.image_url.startsWith('https://cdn.mock.fal.ai/'));
    assert.equal(output.size, 'landscape');
  });

  it('blocks unsafe prompt terms', async () => {
    await assert.rejects(
      runClient({ prompt: 'generate exploit instructions poster' }),
      /FAL_CONTENT_POLICY_BLOCKED/
    );
  });
});
