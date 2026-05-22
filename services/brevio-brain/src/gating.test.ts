import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { evaluateBundleSuggestion, evaluateGating } from './gating.ts';
import type { NormalizedReasoningRequest, PlannerProposal } from './types.ts';

function makeRequest(overrides: Partial<NormalizedReasoningRequest['user_profile']> = {}): NormalizedReasoningRequest {
  return {
    message_text: 'hi',
    deployment_mode: 'cloud',
    user_tier: 'free',
    user_profile: {
      enabled_skills: [],
      granted_categories: [],
      connected_providers: [],
      preferences: {},
      ...overrides
    },
    user_preferences: {}
  } as NormalizedReasoningRequest;
}

function makePlan(actions: Array<{ skill_id: string; action_type?: 'execute_skill' | 'clarify_user' | 'reconcile_results' }>): PlannerProposal {
  return {
    run_id: 'run-1',
    thread_id: 'thread-1',
    planner_provider: 'deterministic',
    planner_model: 'test',
    planner_mode: 'deterministic',
    confidence: 0.9,
    requires_clarification: false,
    actions: actions.map((a, i) => ({
      run_id: 'run-1',
      step_id: `step_${i}`,
      task_id: `task_${i}`,
      attempt: 1,
      intent: 'test.intent',
      skill_id: a.skill_id,
      tool: `${a.skill_id}.op`,
      operation: 'op',
      params: {},
      idempotency_key: 'a'.repeat(16),
      dependencies: [],
      step_dependencies: [],
      rationale: 'test',
      policy: {
        data_class: 'general',
        sensitivity: 'low',
        privacy_mode: 'balanced',
        legal_basis: 'user_request',
        consent_requirement: 'none',
        recipient_verification: 'not_applicable',
        provenance: 'user_message',
        human_review: 'none',
        external_model_egress: 'allow',
        contains_pii: false,
        retention_class: 'standard',
        allowed_processors: ['brevio-core']
      },
      action_type: a.action_type ?? 'execute_skill',
      status: 'pending'
    })),
    policy_summary: {
      privacy_mode: 'balanced',
      data_classes: ['general'],
      contains_pii: false,
      highest_sensitivity: 'low',
      external_model_egress: 'allow',
      requires_consent: false,
      requires_recipient_verification: false,
      human_review_required: false
    },
    risk: { impact: 'low', rollback_plan: 'none' },
    requires_approval: false,
    reasoning: []
  };
}

describe('gating', () => {
  it('passes when all skills are safe-tier and no oauth provider needed', () => {
    const plan = makePlan([{ skill_id: 'todoist' }]);
    const result = evaluateGating(plan, makeRequest());
    assert.equal(result.outcome, 'pass');
  });

  it('passes when skill needs oauth provider AND user has connected_providers', () => {
    const plan = makePlan([{ skill_id: 'google-calendar' }]);
    const result = evaluateGating(plan, makeRequest({ connected_providers: ['google'] }));
    assert.equal(result.outcome, 'pass');
  });

  it('returns requires_credentials when skill needs oauth provider and user has not connected', () => {
    const plan = makePlan([{ skill_id: 'google-calendar' }]);
    const result = evaluateGating(plan, makeRequest());
    assert.equal(result.outcome, 'requires_credentials');
    assert.equal(result.requires_credentials_for?.skill_id, 'google-calendar');
    assert.equal(result.requires_credentials_for?.provider, 'google');
    assert.ok(result.requires_credentials_for?.connect_url.includes('provider=google'));
    assert.ok(result.requires_credentials_for?.scopes.length > 0);
  });

  it('returns requires_consent when skill is in email tier and user has not granted email', () => {
    const plan = makePlan([{ skill_id: 'google-workspace' }]);
    const result = evaluateGating(plan, makeRequest({ connected_providers: ['google'] }));
    assert.equal(result.outcome, 'requires_consent');
    assert.equal(result.requires_consent?.category, 'email');
    assert.equal(result.requires_consent?.skill_id, 'google-workspace');
    assert.ok(result.requires_consent?.prompt_copy.length > 0);
    assert.equal(result.requires_consent?.accept_action.path, '/api/v1/me/consent');
  });

  it('passes when email tier skill is granted and provider connected', () => {
    const plan = makePlan([{ skill_id: 'google-workspace' }]);
    const result = evaluateGating(plan, makeRequest({
      granted_categories: ['email'],
      connected_providers: ['google']
    }));
    assert.equal(result.outcome, 'pass');
  });

  it('skips clarify_user actions', () => {
    const plan = makePlan([{ skill_id: 'google-workspace', action_type: 'clarify_user' }]);
    const result = evaluateGating(plan, makeRequest());
    assert.equal(result.outcome, 'pass');
  });
});

describe('bundle suggestion', () => {
  it('returns undefined when user has no connected providers', () => {
    const plan = makePlan([{ skill_id: 'google-workspace' }]);
    const result = evaluateBundleSuggestion(plan, makeRequest());
    assert.equal(result, undefined);
  });

  it('suggests when user has google connected, action wants google email skill, but email not granted', () => {
    const plan = makePlan([{ skill_id: 'google-workspace' }]);
    const result = evaluateBundleSuggestion(plan, makeRequest({
      connected_providers: ['google'],
      granted_categories: []
    }));
    assert.ok(result, 'expected a bundle suggestion');
    assert.equal(result?.provider, 'google');
    assert.equal(result?.skill_id, 'google-workspace');
    assert.ok(result?.copy.includes('google'));
  });

  it('returns undefined when category is already granted', () => {
    const plan = makePlan([{ skill_id: 'google-workspace' }]);
    const result = evaluateBundleSuggestion(plan, makeRequest({
      connected_providers: ['google'],
      granted_categories: ['email']
    }));
    assert.equal(result, undefined);
  });
});
