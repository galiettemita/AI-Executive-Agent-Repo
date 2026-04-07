import { createHash } from 'node:crypto';

import { getToolDescriptor } from './catalog.js';
import { classifyIntent } from './classify.js';
import { decomposeTask } from './decompose.js';
import { disambiguateSkills } from './disambiguate.js';
import {
  buildActionPolicyMetadata,
  buildExternalPlannerInput,
  buildPlanPolicySummary,
  evaluateExternalPlannerPolicy,
  redactSensitiveText
} from './policy.js';
import type {
  BrainConfig,
  DisambiguationResponse,
  DisambiguationRules,
  IntentClassificationOutput,
  NormalizedReasoningRequest,
  PlannedAction,
  PlannerProposal,
  TaskDescriptor
} from './types.js';

interface ModelAugmentation {
  confidence: number;
  requires_clarification: boolean;
  clarification_question?: string;
  reasoning: string[];
  action_overrides: Array<{
    step_id: string;
    rationale?: string;
    query?: string;
  }>;
}

const MODEL_AUGMENTATION_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  properties: {
    confidence: { type: 'number', minimum: 0, maximum: 1 },
    requires_clarification: { type: 'boolean' },
    clarification_question: { type: 'string' },
    reasoning: {
      type: 'array',
      maxItems: 6,
      items: { type: 'string' }
    },
    action_overrides: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        properties: {
          step_id: { type: 'string' },
          rationale: { type: 'string' },
          query: { type: 'string' }
        },
        required: ['step_id']
      }
    }
  },
  required: ['confidence', 'requires_clarification', 'reasoning', 'action_overrides']
} as const;

function idempotencyKey(taskId: string, skillId: string | undefined, goal: string): string {
  return createHash('sha256').update(`${taskId}::${skillId ?? 'clarify'}::${goal}`).digest('hex');
}

function inferOperation(intent: string, skillId: string | undefined, goal: string): string {
  const descriptor = getToolDescriptor(skillId);
  if (!descriptor) {
    return 'clarify';
  }

  const normalizedGoal = goal.toLowerCase();

  if (intent === 'email.send') {
    if (skillId === 'google-workspace') {
      return 'gmail_send';
    }
    if (skillId === 'outlook') {
      return 'send';
    }
    return 'send';
  }

  if (intent === 'email.search') {
    if (skillId === 'google-workspace') {
      return 'gmail_list';
    }
    if (skillId === 'outlook') {
      return 'inbox_list';
    }
    if (skillId === 'apple-mail-search') {
      if (normalizedGoal.includes('subject')) {
        return 'search_subject';
      }
      if (normalizedGoal.includes('from ')) {
        return 'search_sender';
      }
      return 'search_all';
    }
    if (skillId === 'apple-mail') {
      return 'search';
    }
    if (skillId === 'imap-email') {
      return 'search';
    }
  }

  if (intent === 'calendar.schedule') {
    return 'create';
  }
  if (intent === 'tasks.manage') {
    return 'create';
  }
  if (intent === 'notes.capture') {
    return 'create_note';
  }
  if (intent === 'research.search' || intent === 'shopping.research') {
    return descriptor.operations.includes('search') ? 'search' : descriptor.operations[0] ?? 'search';
  }
  if (intent === 'video.youtube') {
    if (skillId === 'youtube-summarizer') {
      return 'summarize';
    }
    if (skillId === 'video-transcript-downloader') {
      return 'download_transcript';
    }
    return 'search';
  }
  if (intent === 'transport.flight_search') {
    return 'find';
  }
  if (intent === 'transport.flight_tracking') {
    return 'track';
  }
  if (intent === 'places.search') {
    if (skillId === 'google-maps') {
      return 'navigate';
    }
    if (skillId === 'spots') {
      return 'search_all';
    }
    return 'search';
  }
  if (intent === 'finance.expense') {
    return 'analyze';
  }
  if (intent === 'music.playback') {
    return descriptor.operations[0] ?? 'play';
  }
  return descriptor.operations[0] ?? 'execute';
}

function buildParams(skillId: string | undefined, operation: string, goal: string): Record<string, unknown> {
  const params: Record<string, unknown> = {
    action: operation,
    request_segment: goal
  };

  if (operation === 'search' || operation === 'gmail_list' || operation === 'search_all' || operation === 'search_subject' || operation === 'search_sender') {
    params.query = goal;
  }

  if (operation === 'create') {
    if (skillId === 'google-calendar') {
      params.event = { title: goal };
      params.confirmed = false;
    } else {
      params.task = { content: goal };
    }
  }

  if (operation === 'gmail_send' || operation === 'send' || operation === 'reply') {
    params.confirmed = false;
    params.summary = goal;
  }

  return params;
}

function requiresApproval(actions: PlannedAction[]): boolean {
  return actions.some((action) => {
    const descriptor = getToolDescriptor(action.skill_id);
    return (
      (descriptor?.write_operations.includes(action.operation) ?? false) ||
      action.policy.consent_requirement === 'required' ||
      action.policy.human_review === 'required' ||
      action.policy.recipient_verification === 'required'
    );
  });
}

function buildRiskSummary(actions: PlannedAction[]) {
  if (actions.some((action) => action.action_type === 'clarify_user')) {
    return {
      impact: 'Low; the system is waiting for user clarification before executing any external action.',
      rollback_plan: 'No external change has been applied yet.'
    };
  }

  if (actions.some((action) => action.policy.sensitivity === 'critical')) {
    return {
      impact: 'High; the plan touches regulated or critical data classes and requires explicit safeguards before execution.',
      rollback_plan: 'Pause external execution, require human review, and re-issue only the approved steps with the same idempotency keys.'
    };
  }

  if (requiresApproval(actions)) {
    return {
      impact: 'Medium; the plan contains at least one write action against an external system.',
      rollback_plan: 'Hold mutations behind confirmation and replay only the affected task after user review.'
    };
  }

  return {
    impact: 'Low; the plan is read-heavy or advisory.',
    rollback_plan: 'Re-run the affected skill with the same idempotency key if a downstream failure occurs.'
  };
}

function confidenceFromActions(baseConfidence: number, actions: PlannedAction[], requiresClarification: boolean): number {
  let confidence = baseConfidence;
  if (actions.length > 1) {
    confidence -= 0.05;
  }
  if (requiresClarification) {
    confidence = Math.min(confidence, 0.55);
  }
  if (actions.every((action) => action.action_type === 'clarify_user')) {
    confidence = Math.min(confidence, 0.45);
  }
  return Math.max(0.15, Math.min(0.99, Number(confidence.toFixed(2))));
}

function buildClarificationQuestion(classification: IntentClassificationOutput, disambiguation: DisambiguationResponse): string | undefined {
  if (classification.suggested_clarification) {
    return classification.suggested_clarification;
  }
  if (disambiguation.blocked_skills.length > 0) {
    return `I need one of these skills enabled before I act: ${disambiguation.blocked_skills.join(', ')}.`;
  }
  return 'What exactly would you like me to do first?';
}

function sanitizeReasoningLine(line: string): string {
  const normalized = redactSensitiveText(line.trim());
  if (normalized.length === 0) {
    return 'Planner retained the deterministic policy-safe route.';
  }
  return normalized.length > 240 ? `${normalized.slice(0, 239).trimEnd()}…` : normalized;
}

function sanitizeReasoning(lines: string[]): string[] {
  return [...new Set(lines.map((line) => sanitizeReasoningLine(line)).filter((line) => line.length > 0))];
}

function buildClarificationAction(
  request: NormalizedReasoningRequest,
  task: TaskDescriptor,
  intent: string,
  reason: string
): PlannedAction {
  const operation = 'clarify';
  return {
    step_id: `step_${task.id}`,
    task_id: task.id,
    intent,
    operation,
    params: { prompt: reason, request_segment: task.goal },
    idempotency_key: idempotencyKey(task.id, undefined, task.goal),
    dependencies: task.dependencies,
    rationale: sanitizeReasoningLine(reason),
    policy: buildActionPolicyMetadata(request, task.goal, intent, operation, undefined),
    action_type: 'clarify_user',
    status: 'blocked'
  };
}

function buildExecutionAction(
  request: NormalizedReasoningRequest,
  task: TaskDescriptor,
  skillId: string,
  intent: string,
  rationale: string
): PlannedAction {
  const operation = inferOperation(intent, skillId, task.goal);
  return {
    step_id: `step_${task.id}`,
    task_id: task.id,
    intent,
    skill_id: skillId,
    tool: `${skillId}.${operation}`,
    operation,
    params: buildParams(skillId, operation, task.goal),
    idempotency_key: idempotencyKey(task.id, skillId, task.goal),
    dependencies: task.dependencies,
    rationale: sanitizeReasoningLine(rationale),
    policy: buildActionPolicyMetadata(request, task.goal, intent, operation, skillId),
    action_type: 'execute_skill',
    status: 'pending'
  };
}

function extractOutputText(payload: unknown): string | undefined {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
    return undefined;
  }
  const body = payload as Record<string, unknown>;
  const output = body.output;
  if (!Array.isArray(output)) {
    return undefined;
  }
  const fragments: string[] = [];
  for (const item of output) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) {
      continue;
    }
    const content = (item as Record<string, unknown>).content;
    if (!Array.isArray(content)) {
      continue;
    }
    for (const part of content) {
      if (!part || typeof part !== 'object' || Array.isArray(part)) {
        continue;
      }
      if ((part as Record<string, unknown>).type === 'output_text' && typeof (part as Record<string, unknown>).text === 'string') {
        fragments.push((part as Record<string, unknown>).text as string);
      }
    }
  }
  return fragments.length > 0 ? fragments.join('\n').trim() : undefined;
}

async function augmentWithOpenAI(
  proposal: PlannerProposal,
  request: NormalizedReasoningRequest,
  config: BrainConfig,
  fetchImpl: typeof fetch = fetch
): Promise<PlannerProposal> {
  const plannerPolicy = evaluateExternalPlannerPolicy(request, proposal.actions);
  if (!plannerPolicy.allowed) {
    return {
      ...proposal,
      reasoning: sanitizeReasoning([...proposal.reasoning, plannerPolicy.reason])
    };
  }

  const apiKey = process.env.OPENAI_API_KEY;
  if (!apiKey) {
    return {
      ...proposal,
      reasoning: sanitizeReasoning([...proposal.reasoning, 'External planner requested, but credentials are unavailable so the deterministic planner was retained.'])
    };
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), config.plannerTimeoutMs);

  try {
    const response = await fetchImpl(`${config.plannerBaseUrl}/responses`, {
      method: 'POST',
      signal: controller.signal,
      headers: {
        authorization: `Bearer ${apiKey}`,
        'content-type': 'application/json'
      },
      body: JSON.stringify({
        model: config.plannerModel,
        instructions:
          'You are the planner/verifier for a multi-step agent. Return JSON only. Improve confidence, clarification prompts, and action rationale, but do not invent tools, bypass disabled skills, or widen the provided privacy policy.',
        input: [
          {
            role: 'user',
            content: JSON.stringify(buildExternalPlannerInput(request, proposal.actions, proposal.confidence))
          }
        ],
        text: {
          format: {
            type: 'json_schema',
            name: 'brain_plan_augmentation',
            strict: true,
            schema: MODEL_AUGMENTATION_SCHEMA
          }
        },
        max_output_tokens: 1500
      })
    });

    if (!response.ok) {
      return {
        ...proposal,
        reasoning: sanitizeReasoning([...proposal.reasoning, `External planner returned status ${response.status}; deterministic planner retained.`])
      };
    }

    const body = (await response.json()) as unknown;
    const outputText = extractOutputText(body);
    if (!outputText) {
      return {
        ...proposal,
        reasoning: sanitizeReasoning([...proposal.reasoning, 'External planner did not return the expected structured output; deterministic planner retained.'])
      };
    }

    const augmentation = JSON.parse(outputText) as ModelAugmentation;
    const actions = proposal.actions.map((action) => {
      const override = augmentation.action_overrides.find((candidate) => candidate.step_id === action.step_id);
      if (!override) {
        return action;
      }
      return {
        ...action,
        rationale: override.rationale ? sanitizeReasoningLine(override.rationale) : action.rationale,
        params: override.query ? { ...action.params, query: redactSensitiveText(override.query) } : action.params
      };
    });

    return {
      ...proposal,
      planner_mode: 'model_augmented',
      confidence: Math.max(0.15, Math.min(0.99, Number(augmentation.confidence.toFixed(2)))),
      requires_clarification: augmentation.requires_clarification,
      clarification_question: augmentation.clarification_question ?? proposal.clarification_question,
      reasoning: sanitizeReasoning([...proposal.reasoning, ...augmentation.reasoning]),
      actions
    };
  } catch (error) {
    void error;
    return {
      ...proposal,
      reasoning: sanitizeReasoning([...proposal.reasoning, 'External planner was unavailable, so the deterministic planner was retained.'])
    };
  } finally {
    clearTimeout(timeout);
  }
}

export async function buildPlannerProposal(
  request: NormalizedReasoningRequest,
  rules: DisambiguationRules,
  config: BrainConfig,
  fetchImpl?: typeof fetch
): Promise<{
  decomposition: ReturnType<typeof decomposeTask>;
  disambiguation: DisambiguationResponse;
  plan: PlannerProposal;
}> {
  const initialClassification = classifyIntent(request);
  const decomposition = decomposeTask(request.message_text, initialClassification.skills, initialClassification.requires_decomposition);
  const combinedGroupHits = new Set<string>();
  const combinedBlockedSkills = new Set<string>();
  const disambiguationReasoning: string[] = [];
  const actions: PlannedAction[] = [];
  let requiresClarification = initialClassification.clarification_required || decomposition.requires_clarification;

  for (const task of decomposition.tasks) {
    const taskClassification = classifyIntent({
      ...request,
      message_text: task.goal
    });

    const taskDisambiguation = disambiguateSkills(
      {
        message_text: task.goal,
        intent: taskClassification.intent,
        candidate_skills: taskClassification.skills,
        deployment_mode: request.deployment_mode,
        user_tier: request.user_tier,
        user_preferences: request.user_preferences,
        enabled_skills: request.user_profile.enabled_skills
      },
      rules
    );

    for (const group of taskDisambiguation.group_hits) {
      combinedGroupHits.add(group);
    }
    for (const blocked of taskDisambiguation.blocked_skills) {
      combinedBlockedSkills.add(blocked);
    }
    disambiguationReasoning.push(...taskDisambiguation.reasoning);

    if (taskClassification.clarification_required || taskDisambiguation.clarification_required || taskDisambiguation.resolved_skills.length === 0) {
      requiresClarification = true;
      actions.push(
        buildClarificationAction(
          request,
          task,
          taskClassification.intent,
          taskClassification.suggested_clarification ??
            taskDisambiguation.reasoning[0] ??
            `Need clarification before routing "${task.goal}".`
        )
      );
      continue;
    }

    const skillId = taskDisambiguation.resolved_skills[0];
    const reasoning = `Resolved the task to an approved ${taskClassification.intent} execution path.`;
    actions.push(buildExecutionAction(request, task, skillId, taskClassification.intent, reasoning));
  }

  const disambiguation: DisambiguationResponse = {
    resolved_skills: [...new Set(actions.map((action) => action.skill_id).filter((value): value is string => Boolean(value)))],
    group_hits: [...combinedGroupHits],
    blocked_skills: [...combinedBlockedSkills],
    clarification_required: requiresClarification,
    reasoning: disambiguationReasoning.length > 0 ? disambiguationReasoning : ['Planner used direct skill resolution without additional disambiguation groups.']
  };

  let plan: PlannerProposal = {
    planner_provider: config.plannerProvider,
    planner_model: config.plannerModel,
    planner_mode: 'deterministic',
    confidence: confidenceFromActions(initialClassification.confidence, actions, requiresClarification),
    requires_clarification: requiresClarification,
    clarification_question: requiresClarification ? buildClarificationQuestion(initialClassification, disambiguation) : undefined,
    actions,
    policy_summary: buildPlanPolicySummary(request, actions),
    risk: buildRiskSummary(actions),
    requires_approval: requiresApproval(actions),
    reasoning: sanitizeReasoning([
      initialClassification.reasoning,
      ...decomposition.reasoning,
      ...disambiguation.reasoning
    ])
  };

  if (config.plannerProvider === 'openai_responses') {
    plan = await augmentWithOpenAI(plan, request, config, fetchImpl);
  }

  return {
    decomposition,
    disambiguation,
    plan
  };
}
