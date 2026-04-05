import type { PlannerProposal, ProcessRequest, SkillResult, VerificationResult } from './types.js';

function collectPlannedSkillIds(plan: PlannerProposal): Set<string> {
  return new Set(plan.actions.map((action) => action.skill_id).filter((value): value is string => Boolean(value)));
}

export function verifyPlan(plan: PlannerProposal, skillResults: SkillResult[] | undefined, request: ProcessRequest): VerificationResult {
  const issues: string[] = [];
  const warnings: string[] = [];

  if (plan.actions.length === 0) {
    issues.push('plan_contains_no_actions');
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

  return {
    valid: issues.length === 0,
    issues,
    warnings
  };
}
