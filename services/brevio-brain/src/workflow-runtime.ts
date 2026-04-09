import type { BrainConfig, ExecutionStatus, PlannerProposal } from './types.js';

interface RuntimeStepRecord {
  step_id?: unknown;
  state_key?: unknown;
  status?: unknown;
}

interface RuntimeListStepsResponse {
  steps?: unknown;
}

interface WorkflowRuntimeStep {
  stepId: string;
  stateKey: string;
  status: string;
}

export interface BrainWorkflowSyncResult {
  delegated: boolean;
  transitioned: string[];
  warning?: string;
}

export interface BrainWorkflowPlanRegistrationResult {
  delegated: boolean;
  registeredSteps: number;
  warning?: string;
}

export interface BrainWorkflowAnnotationResult {
  delegated: boolean;
  warning?: string;
}

function asString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function baseUrl(config: Pick<BrainConfig, 'temporalWorkerBaseUrl'>): string | undefined {
  const value = config.temporalWorkerBaseUrl?.trim();
  return value && value.length > 0 ? value.replace(/\/+$/, '') : undefined;
}

async function listRunSteps(
  runId: string,
  config: Pick<BrainConfig, 'temporalWorkerBaseUrl' | 'temporalWorkerTimeoutMs'>,
  fetchImpl: typeof fetch
): Promise<WorkflowRuntimeStep[]> {
  const url = `${baseUrl(config)}/api/v1/temporal-worker/runs/${encodeURIComponent(runId)}/steps`;
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), config.temporalWorkerTimeoutMs);

  try {
    const response = await fetchImpl(url, {
      method: 'GET',
      signal: controller.signal
    });

    if (!response.ok) {
      throw new Error(`runtime_steps_status_${response.status}`);
    }

    const payload = (await response.json()) as RuntimeListStepsResponse;
    if (!Array.isArray(payload.steps)) {
      return [];
    }

    return payload.steps
      .map((item) => {
        const raw = item as RuntimeStepRecord;
        const stepId = asString(raw.step_id);
        const stateKey = asString(raw.state_key);
        const status = asString(raw.status);
        if (!stepId || !stateKey || !status) {
          return undefined;
        }
        return {
          stepId,
          stateKey,
          status
        };
      })
      .filter((step): step is WorkflowRuntimeStep => Boolean(step));
  } finally {
    clearTimeout(timeout);
  }
}

async function transitionStep(
  runId: string,
  stepId: string,
  status: 'RUNNING' | 'COMPLETED',
  config: Pick<BrainConfig, 'temporalWorkerBaseUrl' | 'temporalWorkerTimeoutMs'>,
  fetchImpl: typeof fetch
): Promise<void> {
  const url = `${baseUrl(config)}/api/v1/temporal-worker/runs/${encodeURIComponent(runId)}/steps/${encodeURIComponent(stepId)}/transition`;
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), config.temporalWorkerTimeoutMs);

  try {
    const response = await fetchImpl(url, {
      method: 'POST',
      signal: controller.signal,
      headers: {
        'content-type': 'application/json'
      },
      body: JSON.stringify({ status })
    });

    if (!response.ok) {
      throw new Error(`runtime_transition_status_${response.status}`);
    }
  } finally {
    clearTimeout(timeout);
  }
}

function executionPlanUrl(runId: string, config: Pick<BrainConfig, 'temporalWorkerBaseUrl'>): string {
  return `${baseUrl(config)}/api/v1/temporal-worker/runs/${encodeURIComponent(runId)}/execution-plan`;
}

function runMetadataUrl(runId: string, config: Pick<BrainConfig, 'temporalWorkerBaseUrl'>): string {
  return `${baseUrl(config)}/api/v1/temporal-worker/runs/${encodeURIComponent(runId)}/metadata`;
}

export async function syncProcessRunState(
  runId: string | undefined,
  executionStatus: ExecutionStatus,
  config: Pick<BrainConfig, 'temporalWorkerBaseUrl' | 'temporalWorkerTimeoutMs'>,
  fetchImpl: typeof fetch = fetch
): Promise<BrainWorkflowSyncResult> {
  if (!runId || !baseUrl(config)) {
    return { delegated: false, transitioned: [] };
  }

  try {
    const steps = await listRunSteps(runId, config, fetchImpl);
    const stepMap = new Map(steps.map((step) => [step.stateKey, step]));
    const transitioned: string[] = [];

    const ensureRunning = async (stateKey: string) => {
      const step = stepMap.get(stateKey);
      if (!step || step.status === 'RUNNING' || step.status === 'COMPLETED') {
        return;
      }
      await transitionStep(runId, step.stepId, 'RUNNING', config, fetchImpl);
      step.status = 'RUNNING';
      transitioned.push(`${stateKey}:RUNNING`);
    };

    const ensureCompleted = async (stateKey: string) => {
      const step = stepMap.get(stateKey);
      if (!step || step.status === 'COMPLETED') {
        return;
      }
      if (step.status !== 'RUNNING') {
        await transitionStep(runId, step.stepId, 'RUNNING', config, fetchImpl);
        step.status = 'RUNNING';
        transitioned.push(`${stateKey}:RUNNING`);
      }
      await transitionStep(runId, step.stepId, 'COMPLETED', config, fetchImpl);
      step.status = 'COMPLETED';
      transitioned.push(`${stateKey}:COMPLETED`);
    };

    await ensureCompleted('RECEIVED');
    await ensureCompleted('CLASSIFYING');

    if (executionStatus === 'verification_failed') {
      await ensureRunning('DECOMPOSING');
      return {
        delegated: true,
        transitioned
      };
    }

    await ensureCompleted('DECOMPOSING');

    if (executionStatus === 'dispatch_ready') {
      await ensureRunning('EXECUTING');
    }

    if (executionStatus === 'completed') {
      await ensureCompleted('EXECUTING');
      await ensureCompleted('AGGREGATING');
    }

    return {
      delegated: true,
      transitioned
    };
  } catch (error) {
    return {
      delegated: false,
      transitioned: [],
      warning:
        error instanceof Error && error.name === 'AbortError'
          ? 'temporal_worker_sync_timeout'
          : 'temporal_worker_sync_unavailable'
    };
  }
}

export async function registerExecutionPlan(
  runId: string | undefined,
  plan: PlannerProposal,
  config: Pick<BrainConfig, 'temporalWorkerBaseUrl' | 'temporalWorkerTimeoutMs'>,
  fetchImpl: typeof fetch = fetch
): Promise<BrainWorkflowPlanRegistrationResult> {
  if (!runId || !baseUrl(config)) {
    return { delegated: false, registeredSteps: 0 };
  }

  const executeActions = plan.actions.filter(
    (action): action is PlannerProposal['actions'][number] & { skill_id: string } =>
      action.action_type === 'execute_skill' && typeof action.skill_id === 'string' && action.skill_id.trim().length > 0
  );

  if (executeActions.length === 0) {
    return { delegated: false, registeredSteps: 0 };
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), config.temporalWorkerTimeoutMs);
  const taskToStepIds = new Map<string, string[]>();
  for (const action of executeActions) {
    const existing = taskToStepIds.get(action.task_id) ?? [];
    existing.push(action.step_id);
    taskToStepIds.set(action.task_id, existing);
  }

  try {
    const response = await fetchImpl(executionPlanUrl(runId, config), {
      method: 'POST',
      signal: controller.signal,
      headers: {
        'content-type': 'application/json'
      },
      body: JSON.stringify({
        steps: executeActions.map((action) => ({
          planner_step_id: action.step_id,
          planner_task_id: action.task_id,
          title: action.rationale,
          skill_id: action.skill_id,
          operation: action.operation,
          dependencies: [...new Set([
            ...action.dependencies.flatMap((taskId) => taskToStepIds.get(taskId) ?? []),
            ...(action.step_dependencies ?? [])
          ])],
          metadata: {
            tool: action.tool,
            intent: action.intent,
            idempotency_key: action.idempotency_key,
            policy: action.policy,
            action_status: action.status
          }
        }))
      })
    });

    if (!response.ok) {
      return {
        delegated: false,
        registeredSteps: 0,
        warning: `temporal_worker_plan_register_failed_status_${response.status}`
      };
    }

    const payload = await response.json() as { total?: unknown };
    return {
      delegated: true,
      registeredSteps: typeof payload.total === 'number' ? payload.total : executeActions.length
    };
  } catch (error) {
    return {
      delegated: false,
      registeredSteps: 0,
      warning:
        error instanceof Error && error.name === 'AbortError'
          ? 'temporal_worker_plan_register_timeout'
          : 'temporal_worker_plan_register_unavailable'
    };
  } finally {
    clearTimeout(timeout);
  }
}

export async function annotateRunVerification(
  runId: string | undefined,
  verification: { valid: boolean; issues: string[]; warnings: string[] },
  config: Pick<BrainConfig, 'temporalWorkerBaseUrl' | 'temporalWorkerTimeoutMs'>,
  fetchImpl: typeof fetch = fetch
): Promise<BrainWorkflowAnnotationResult> {
  if (!runId || !baseUrl(config)) {
    return { delegated: false };
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), config.temporalWorkerTimeoutMs);

  try {
    const response = await fetchImpl(runMetadataUrl(runId, config), {
      method: 'POST',
      signal: controller.signal,
      headers: {
        'content-type': 'application/json'
      },
      body: JSON.stringify({
        metadata: {
          verification_valid: verification.valid,
          verification_issues: verification.issues,
          verification_warnings: verification.warnings,
          verification_recorded_at: new Date().toISOString()
        }
      })
    });
    if (!response.ok) {
      return {
        delegated: false,
        warning: `temporal_worker_verification_annotation_failed_status_${response.status}`
      };
    }
    return { delegated: true };
  } catch (error) {
    return {
      delegated: false,
      warning:
        error instanceof Error && error.name === 'AbortError'
          ? 'temporal_worker_verification_annotation_timeout'
          : 'temporal_worker_verification_annotation_unavailable'
    };
  } finally {
    clearTimeout(timeout);
  }
}
