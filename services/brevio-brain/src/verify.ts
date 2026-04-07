import { getToolDescriptor } from './catalog.js';
import type { PlannerProposal, ProcessRequest, SkillResult, VerificationResult } from './types.js';

function collectPlannedSkillIds(plan: PlannerProposal): Set<string> {
  return new Set(plan.actions.map((action) => action.skill_id).filter((value): value is string => Boolean(value)));
}

function expectedApproval(plan: PlannerProposal): boolean {
  return plan.actions.some(
    (action) =>
      (Boolean(action.policy) && (getToolDescriptor(action.skill_id)?.write_operations.includes(action.operation) ?? false)) ||
      action.policy?.consent_requirement === 'required' ||
      action.policy?.human_review === 'required' ||
      action.policy?.recipient_verification === 'required'
  );
}

export function verifyPlan(plan: PlannerProposal, skillResults: SkillResult[] | undefined, request: ProcessRequest): VerificationResult {
  const issues: string[] = [];
  const warnings: string[] = [];
  const enabledSkills = request.user_profile?.enabled_skills ?? [];

  if (plan.actions.length === 0) {
    issues.push('plan_contains_no_actions');
  }

  if (!plan.policy_summary) {
    issues.push('missing_plan_policy_summary');
  }

  for (const action of plan.actions) {
    if (action.action_type === 'execute_skill' && !action.skill_id) {
      issues.push(`missing_skill_for_${action.step_id}`);
    }
    if (action.action_type === 'execute_skill' && !action.tool) {
      issues.push(`missing_tool_for_${action.step_id}`);
    }
    if (action.idempotency_key.trim().length < 16) {
      issues.push(`weak_idempotency_key_for_${action.step_id}`);
    }
    if (!action.policy) {
      issues.push(`missing_policy_for_${action.step_id}`);
      continue;
    }
    if (!Array.isArray(action.policy.allowed_processors) || action.policy.allowed_processors.length === 0) {
      issues.push(`missing_allowed_processors_for_${action.step_id}`);
    }
    if (!action.policy.legal_basis) {
      issues.push(`missing_legal_basis_for_${action.step_id}`);
    }
    if (!action.policy.provenance) {
      issues.push(`missing_provenance_for_${action.step_id}`);
    }
    if (!action.policy.retention_class) {
      issues.push(`missing_retention_class_for_${action.step_id}`);
    }
    if (action.action_type === 'execute_skill' && action.skill_id && !enabledSkills.includes(action.skill_id)) {
      issues.push(`skill_not_enabled_for_${action.step_id}`);
    }
    if (
      action.policy.recipient_verification === 'required' &&
      !['send', 'reply', 'gmail_send'].includes(action.operation)
    ) {
      warnings.push(`recipient_verification_marked_for_non_message_action:${action.step_id}`);
    }
  }

  if (plan.requires_approval !== expectedApproval(plan)) {
    issues.push('requires_approval_mismatch');
  }
  if (plan.policy_summary) {
    const requiresConsent = plan.actions.some((action) => action.policy?.consent_requirement !== undefined && action.policy.consent_requirement !== 'none');
    const requiresRecipientVerification = plan.actions.some((action) => action.policy?.recipient_verification === 'required');
    const humanReviewRequired = plan.actions.some((action) => action.policy?.human_review === 'required');
    if (plan.policy_summary.requires_consent !== requiresConsent) {
      issues.push('policy_summary_requires_consent_mismatch');
    }
    if (plan.policy_summary.requires_recipient_verification !== requiresRecipientVerification) {
      issues.push('policy_summary_requires_recipient_verification_mismatch');
    }
    if (plan.policy_summary.human_review_required !== humanReviewRequired) {
      issues.push('policy_summary_human_review_mismatch');
    }
    if (plan.planner_mode === 'model_augmented' && plan.policy_summary.external_model_egress === 'deny') {
      issues.push('model_augmented_plan_violates_external_egress_policy');
    }
  }

  if (!skillResults || skillResults.length === 0) {
    if (plan.actions.some((action) => action.action_type === 'execute_skill')) {
      warnings.push('process_response_is_dispatch_only_until_real_skill_results_arrive');
    }
  } else {
    const plannedSkills = collectPlannedSkillIds(plan);
    for (const result of skillResults) {
      if (!plannedSkills.has(result.skill_id)) {
        warnings.push(`unplanned_skill_result:${result.skill_id}`);
      }
    }
  }

  if (request.message_text.trim().length === 0) {
    issues.push('empty_message_text');
  }

  if (plan.actions.some((action) => action.action_type === 'execute_skill') && enabledSkills.length === 0) {
    issues.push('execute_plan_without_enabled_skill_inventory');
  }

  return {
    valid: issues.length === 0,
    issues,
    warnings
  };
}
