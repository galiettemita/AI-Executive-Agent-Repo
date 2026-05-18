import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { runClient } from '../client.js';

describe('google-maps unit', () => {
  it('returns deterministic route payload', async () => {
    const output = await runClient({
      origin: 'San Francisco, CA',
      destination: 'SFO Airport',
      mode: 'driving'
    });

    assert.ok(output.distance_m > 0);
    assert.ok(output.duration_s > 0);
    assert.equal(output.mode, 'driving');
    assert.equal(output.steps.length, 3);
  });
});
