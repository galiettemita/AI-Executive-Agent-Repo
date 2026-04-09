import { getToolDescriptor } from './catalog.js';
import type { PlannerProposal, ProcessRequest, SkillResult, VerificationResult } from './types.js';

function collectPlannedSkillIds(plan: PlannerProposal): Set<string> {
  return new Set(plan.actions.map((action) => action.skill_id).filter((value): value is string => Boolean(value)));
}

function collectExecuteActions(plan: PlannerProposal) {
  return plan.actions.filter(
    (action): action is PlannerProposal['actions'][number] & { skill_id: string } =>
      action.action_type === 'execute_skill' && Boolean(action.skill_id)
  );
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
  const executeActions = collectExecuteActions(plan);
  const executeActionsByStep = new Map(executeActions.map((action) => [action.step_id, action]));
  const executeSkillCounts = executeActions.reduce<Map<string, number>>((counts, action) => {
    counts.set(action.skill_id, (counts.get(action.skill_id) ?? 0) + 1);
    return counts;
  }, new Map());

  if (plan.actions.length === 0) {
    issues.push('plan_contains_no_actions');
  }

  if (request.run_id && plan.run_id !== request.run_id) {
    issues.push('plan_run_id_mismatch');
  }

  if (request.thread_id && plan.thread_id !== request.thread_id) {
    issues.push('plan_thread_id_mismatch');
  }

  if (!plan.policy_summary) {
    issues.push('missing_plan_policy_summary');
  }

  for (const action of plan.actions) {
    if (!action.run_id || action.run_id !== plan.run_id) {
      issues.push(`missing_run_id_for_${action.step_id}`);
    }
    if (!action.attempt || action.attempt < 1) {
      issues.push(`invalid_attempt_for_${action.step_id}`);
    }
    if (action.action_type === 'execute_skill' && !action.skill_id) {
      issues.push(`missing_skill_for_${action.step_id}`);
    }
    if (action.action_type === 'execute_skill' && !action.tool) {
      issues.push(`missing_tool_for_${action.step_id}`);
    }
    if (action.action_type === 'reconcile_results' && (!action.step_dependencies || action.step_dependencies.length < 2)) {
      issues.push(`missing_specialist_dependencies_for_${action.step_id}`);
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
      if (result.run_id && result.run_id !== plan.run_id) {
        warnings.push(`run_result_mismatch:${result.skill_id}`);
      }
      if (result.step_id) {
        const plannedAction = executeActionsByStep.get(result.step_id);
        if (!plannedAction) {
          warnings.push(`unplanned_step_result:${result.step_id}`);
          continue;
        }
        if (plannedAction.skill_id !== result.skill_id) {
          warnings.push(`step_skill_mismatch:${result.step_id}`);
        }
        if (result.task_id && plannedAction.task_id !== result.task_id) {
          warnings.push(`step_task_mismatch:${result.step_id}`);
        }
      } else if ((executeSkillCounts.get(result.skill_id) ?? 0) > 1) {
        warnings.push(`uncorrelated_skill_result:${result.skill_id}`);
      }
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
