import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { verifyPlan } from './verify.js';

describe('verifyPlan', () => {
  it('warns when a plan has not executed yet', () => {
    const verification = verifyPlan(
      {
        planner_provider: 'deterministic',
        planner_model: 'gpt-5.2',
        planner_mode: 'deterministic',
        confidence: 0.7,
        requires_clarification: false,
        actions: [
          {
            step_id: 'step_t1',
            task_id: 't1',
            intent: 'music.playback',
            skill_id: 'spotify-web-api',
            tool: 'spotify-web-api.play',
            operation: 'play',
            params: { action: 'play' },
            idempotency_key: '1234567890abcdef1234567890abcdef',
            dependencies: [],
            rationale: 'Play music',
            action_type: 'execute_skill',
            status: 'pending'
          }
        ],
        risk: {
          impact: 'Low',
          rollback_plan: 'Retry'
        },
        requires_approval: false,
        reasoning: ['deterministic plan']
      },
      [],
      { message_text: 'play music' }
    );

    assert.equal(verification.valid, true);
    assert.match(verification.warnings.join(' '), /dispatch_only_until_real_skill_results_arrive/);
  });
});
