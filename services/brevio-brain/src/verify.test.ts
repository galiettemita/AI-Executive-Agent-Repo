import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { verifyPlan } from './verify.js';

const basePolicy = {
  data_class: 'general' as const,
  sensitivity: 'low' as const,
  privacy_mode: 'strict' as const,
  legal_basis: 'user_request' as const,
  consent_requirement: 'none' as const,
  recipient_verification: 'not_applicable' as const,
  provenance: 'user_message' as const,
  human_review: 'none' as const,
  external_model_egress: 'redacted_only' as const,
  contains_pii: false,
  retention_class: 'ephemeral' as const,
  allowed_processors: ['brain', 'policy', 'approved_connector']
};

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
            policy: basePolicy,
            action_type: 'execute_skill',
            status: 'pending'
          }
        ],
        policy_summary: {
          privacy_mode: 'strict',
          data_classes: ['general'],
          contains_pii: false,
          highest_sensitivity: 'low',
          external_model_egress: 'redacted_only',
          requires_consent: false,
          requires_recipient_verification: false,
          human_review_required: false
        },
        risk: {
          impact: 'Low',
          rollback_plan: 'Retry'
        },
        requires_approval: false,
        reasoning: ['deterministic plan']
      },
      [],
      {
        message_text: 'play music',
        user_profile: {
          enabled_skills: ['spotify-web-api']
        }
      }
    );

    assert.equal(verification.valid, true);
    assert.match(verification.warnings.join(' '), /dispatch_only_until_real_skill_results_arrive/);
  });

  it('fails closed when policy metadata is missing', () => {
    const verification = verifyPlan(
      {
        planner_provider: 'deterministic',
        planner_model: 'gpt-5.2',
        planner_mode: 'deterministic',
        confidence: 0.4,
        requires_clarification: false,
        actions: [
          {
            step_id: 'step_t1',
            task_id: 't1',
            intent: 'email.send',
            skill_id: 'google-workspace',
            tool: 'google-workspace.gmail_send',
            operation: 'gmail_send',
            params: { action: 'gmail_send' },
            idempotency_key: '1234567890abcdef1234567890abcdef',
            dependencies: [],
            rationale: 'Send an email',
            action_type: 'execute_skill',
            status: 'pending'
          } as never
        ],
        policy_summary: {
          privacy_mode: 'strict',
          data_classes: ['communications'],
          contains_pii: false,
          highest_sensitivity: 'moderate',
          external_model_egress: 'redacted_only',
          requires_consent: true,
          requires_recipient_verification: true,
          human_review_required: true
        },
        risk: {
          impact: 'Medium',
          rollback_plan: 'Retry'
        },
        requires_approval: true,
        reasoning: ['deterministic plan']
      },
      [],
      {
        message_text: 'send email',
        user_profile: {
          enabled_skills: ['google-workspace']
        }
      }
    );

    assert.equal(verification.valid, false);
    assert.match(verification.issues.join(' '), /missing_policy_for_step_t1/);
  });
});
