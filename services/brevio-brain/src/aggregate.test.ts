import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { aggregateResults } from './aggregate.js';

describe('aggregateResults', () => {
  it('summarizes real skill outputs and warnings', () => {
    const result = aggregateResults({
      channel: 'API',
      user_profile: { communication_style: 'balanced' },
      skill_results: [
        {
          skill_id: 'spotify-web-api',
          status: 'SUCCESS',
          data: { summary: 'Playback started.' }
        },
        {
          skill_id: 'google-calendar',
          status: 'FAILED',
          error: { code: 'CONFIRMATION_REQUIRED', message: 'Need explicit confirmation.' }
        }
      ]
    });

    assert.equal(result.completion_ratio, 0.5);
    assert.match(result.response_text, /Playback started/);
    assert.equal(result.warnings.length, 1);
  });
});
