import { randomUUID } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import path from 'node:path';

export type WorkflowType = 'message-processing' | 'daily-rhythm';
export type WorkflowStatus = 'RUNNING' | 'COMPLETED' | 'FAILED' | 'DEAD_LETTER';
export type WorkflowTaskStatus = 'PENDING' | 'RUNNING' | 'COMPLETED' | 'FAILED' | 'DEAD_LETTER';
export type WorkflowStepStatus = 'PENDING' | 'READY' | 'RUNNING' | 'COMPLETED' | 'FAILED' | 'DEAD_LETTER';

export interface WorkflowArtifact {
  artifact_id: string;
  type: string;
  uri?: string;
  inline_data?: unknown;
}

export interface WorkflowRunRecord {
  run_id: string;
  workflow_id: string;
  workflow_type: WorkflowType;
  user_id?: string;
  status: WorkflowStatus;
  states: string[];
  current_state: string;
  started_at: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
  task_ids: string[];
  metadata: Record<string, unknown>;
}

export interface WorkflowTaskRecord {
  task_id: string;
  run_id: string;
  name: string;
  sequence: number;
  status: WorkflowTaskStatus;
  step_ids: string[];
  created_at: string;
  updated_at: string;
  completed_at?: string;
  metadata: Record<string, unknown>;
}

export interface WorkflowStepRecord {
  step_id: string;
  run_id: string;
  task_id: string;
  state_key: string;
  title: string;
  sequence: number;
  status: WorkflowStepStatus;
  attempt: number;
  depends_on_step_ids?: string[];
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at?: string;
  artifacts?: WorkflowArtifact[];
  last_error?: {
    code: string;
    message: string;
  };
  metadata: Record<string, unknown>;
}

export interface WorkflowStepBlueprint {
  state_key: string;
  title: string;
  status: WorkflowStepStatus;
  depends_on_step_ids?: string[];
  metadata?: Record<string, unknown>;
}

export interface WorkflowExecutionStepBlueprint {
  planner_step_id: string;
  planner_task_id: string;
  title: string;
  skill_id?: string;
  operation?: string;
  dependencies?: string[];
  metadata?: Record<string, unknown>;
}

export interface CreateWorkflowRunInput {
  run_id?: string;
  workflow_id: string;
  workflow_type: WorkflowType;
  user_id?: string;
  status: WorkflowStatus;
  current_state: string;
  started_at: string;
  completed_at?: string;
  metadata?: Record<string, unknown>;
  steps: WorkflowStepBlueprint[];
}

interface WorkflowSnapshot {
  version: 1;
  runs: WorkflowRunRecord[];
  tasks: WorkflowTaskRecord[];
  steps: WorkflowStepRecord[];
}

type NonTerminalWorkflowStepStatus = Exclude<WorkflowStepStatus, 'COMPLETED' | 'FAILED' | 'DEAD_LETTER'>;

interface StepTransitionInput {
  run_id: string;
  step_id: string;
  status: WorkflowStepStatus;
  artifacts?: WorkflowArtifact[];
  error?: {
    code: string;
    message: string;
  };
  metadata?: Record<string, unknown>;
}

function nowIso(): string {
  return new Date().toISOString();
}

function taskStatusFromStepStatus(status: WorkflowStepStatus): WorkflowTaskStatus {
  switch (status) {
    case 'COMPLETED':
      return 'COMPLETED';
    case 'FAILED':
      return 'FAILED';
    case 'DEAD_LETTER':
      return 'DEAD_LETTER';
    case 'RUNNING':
    case 'READY':
      return 'RUNNING';
    case 'PENDING':
    default:
      return 'PENDING';
  }
}

function emptySnapshot(): WorkflowSnapshot {
  return {
    version: 1,
    runs: [],
    tasks: [],
    steps: []
  };
}

function sortBySequence<T extends { sequence: number }>(records: T[]): T[] {
  return [...records].sort((left, right) => left.sequence - right.sequence);
}

function sanitizeIdentifier(value: string): string {
  return value.replace(/[^a-zA-Z0-9_-]+/g, '_');
}

function uniqueStrings(values: string[]): string[] {
  return [...new Set(values.filter((value) => value.trim().length > 0))];
}

export class WorkflowStore {
  private readonly snapshot: WorkflowSnapshot;
  private readonly filePath: string;

  constructor(filePath: string) {
    this.filePath = filePath;
    this.snapshot = this.loadSnapshot();
  }

  stats(): { runs: number; tasks: number; steps: number; file_path: string } {
    return {
      runs: this.snapshot.runs.length,
      tasks: this.snapshot.tasks.length,
      steps: this.snapshot.steps.length,
      file_path: this.filePath
    };
  }

  listRuns(): WorkflowRunRecord[] {
    return [...this.snapshot.runs].sort((left, right) => left.started_at.localeCompare(right.started_at));
  }

  getRun(runId: string): WorkflowRunRecord | undefined {
    return this.snapshot.runs.find((run) => run.run_id === runId);
  }

  getTask(taskId: string): WorkflowTaskRecord | undefined {
    return this.snapshot.tasks.find((task) => task.task_id === taskId);
  }

  getRunTask(runId: string, taskId: string): WorkflowTaskRecord | undefined {
    return this.snapshot.tasks.find((task) => task.run_id === runId && task.task_id === taskId);
  }

  listTasks(runId: string): WorkflowTaskRecord[] {
    return sortBySequence(this.snapshot.tasks.filter((task) => task.run_id === runId));
  }

  listSteps(runId: string): WorkflowStepRecord[] {
    return sortBySequence(this.snapshot.steps.filter((step) => step.run_id === runId));
  }

  getPlannerStep(runId: string, plannerStepId: string): WorkflowStepRecord | undefined {
    return this.snapshot.steps.find(
      (step) => step.run_id === runId && step.metadata?.planner_step_id === plannerStepId
    );
  }

  createRun(input: CreateWorkflowRunInput): WorkflowRunRecord {
    const createdAt = input.started_at || nowIso();
    const runId = input.run_id ?? randomUUID();
    const taskIds: string[] = [];
    let previousStepId: string | undefined;

    for (const [index, blueprint] of input.steps.entries()) {
      const taskId = `task_${runId}_${index + 1}`;
      const stepId = `step_${runId}_${index + 1}`;
      const taskStatus = taskStatusFromStepStatus(blueprint.status);

      taskIds.push(taskId);
      this.snapshot.tasks.push({
        task_id: taskId,
        run_id: runId,
        name: blueprint.title,
        sequence: index + 1,
        status: taskStatus,
        step_ids: [stepId],
        created_at: createdAt,
        updated_at: createdAt,
        completed_at: taskStatus === 'COMPLETED' ? createdAt : undefined,
        metadata: {
          state_key: blueprint.state_key,
          ...(blueprint.metadata ?? {})
        }
      });

      this.snapshot.steps.push({
        step_id: stepId,
        run_id: runId,
        task_id: taskId,
        state_key: blueprint.state_key,
        title: blueprint.title,
        sequence: index + 1,
        status: blueprint.status,
        attempt: blueprint.status === 'RUNNING' ? 1 : 0,
        depends_on_step_ids:
          blueprint.depends_on_step_ids && blueprint.depends_on_step_ids.length > 0
            ? [...blueprint.depends_on_step_ids]
            : previousStepId
              ? [previousStepId]
              : [],
        created_at: createdAt,
        updated_at: createdAt,
        started_at: blueprint.status === 'RUNNING' ? createdAt : undefined,
        completed_at: blueprint.status === 'COMPLETED' || blueprint.status === 'FAILED' || blueprint.status === 'DEAD_LETTER' ? createdAt : undefined,
        metadata: { ...(blueprint.metadata ?? {}) }
      });

      previousStepId = stepId;
    }

    const run: WorkflowRunRecord = {
      run_id: runId,
      workflow_id: input.workflow_id,
      workflow_type: input.workflow_type,
      user_id: input.user_id,
      status: input.status,
      states: input.steps.map((step) => step.state_key),
      current_state: input.current_state,
      started_at: createdAt,
      completed_at: input.completed_at,
      created_at: createdAt,
      updated_at: createdAt,
      task_ids: taskIds,
      metadata: { ...(input.metadata ?? {}) }
    };

    this.snapshot.runs.push(run);
    this.persist();
    return run;
  }

  registerExecutionPlan(runId: string, steps: WorkflowExecutionStepBlueprint[]): WorkflowStepRecord[] {
    const run = this.requireRun(runId);
    if (steps.length === 0) {
      return [];
    }

    const existingSteps = this.listSteps(runId);
    const executionPhase = existingSteps.find((step) => step.state_key === 'EXECUTING');
    if (!executionPhase) {
      throw new Error('executing_phase_not_found');
    }

    const aggregatePhase = existingSteps.find((step) => step.state_key === 'AGGREGATING');
    const insertSequence = aggregatePhase?.sequence ?? (Math.max(...existingSteps.map((step) => step.sequence), 0) + 1);

    const createdAt = nowIso();
    const internalStepIds = new Map<string, string>();
    const plannerTaskRecords = new Map<string, WorkflowTaskRecord>();
    const newPlannerTaskIds = uniqueStrings(
      steps
        .map((step) => step.planner_task_id)
        .filter((plannerTaskId) => !this.findPlannerTaskRecord(runId, plannerTaskId))
    );
    const newTaskSequenceOffsets = new Map<string, number>(
      newPlannerTaskIds.map((plannerTaskId, index) => [plannerTaskId, index])
    );
    const registered: WorkflowStepRecord[] = [];
    const newSteps = steps.filter((step) => !this.getPlannerStep(runId, step.planner_step_id));

    for (const task of this.listTasks(runId)) {
      const plannerTaskId = typeof task.metadata?.planner_task_id === 'string' ? task.metadata.planner_task_id : undefined;
      if (plannerTaskId) {
        plannerTaskRecords.set(plannerTaskId, task);
      }
    }

    for (const step of steps) {
      const existing = this.getPlannerStep(runId, step.planner_step_id);
      if (existing) {
        internalStepIds.set(step.planner_step_id, existing.step_id);
      } else {
        internalStepIds.set(step.planner_step_id, `step_${runId}_${sanitizeIdentifier(step.planner_step_id)}`);
      }
    }

    if (newSteps.length > 0) {
      for (const step of this.snapshot.steps) {
        if (step.run_id === runId && step.sequence >= insertSequence) {
          step.sequence += newSteps.length;
        }
      }
      for (const task of this.snapshot.tasks) {
        if (task.run_id === runId && task.sequence >= insertSequence) {
          task.sequence += newPlannerTaskIds.length;
        }
      }
    }

    for (const [index, step] of steps.entries()) {
      const existing = this.getPlannerStep(runId, step.planner_step_id);
      const dependsOnPlannerSteps = [...new Set(step.dependencies?.filter((dependency) => dependency.trim().length > 0) ?? [])];
      const dependsOnStepIds = dependsOnPlannerSteps
        .map((dependency) => internalStepIds.get(dependency))
        .filter((dependency): dependency is string => Boolean(dependency));

      if (existing) {
        existing.depends_on_step_ids = dependsOnStepIds;
        existing.title = step.title;
        existing.metadata = {
          ...existing.metadata,
          planner_task_id: step.planner_task_id,
          planner_step_id: step.planner_step_id,
          dependency_planner_step_ids: dependsOnPlannerSteps,
          phase: 'EXECUTING',
          skill_id: step.skill_id,
          operation: step.operation,
          ...(step.metadata ?? {})
        };
        existing.updated_at = createdAt;
        registered.push(existing);
        continue;
      }

      const taskId = `task_${runId}_${sanitizeIdentifier(step.planner_task_id)}`;
      const stepId = internalStepIds.get(step.planner_step_id)!;
      const sequence = insertSequence + index;
      const initialStatus: WorkflowStepStatus = dependsOnStepIds.length > 0 ? 'PENDING' : 'READY';
      const taskRecord =
        plannerTaskRecords.get(step.planner_task_id) ??
        this.createPlannerTaskRecord(runId, step, insertSequence + (newTaskSequenceOffsets.get(step.planner_task_id) ?? 0), createdAt);
      if (!plannerTaskRecords.has(step.planner_task_id)) {
        this.snapshot.tasks.push(taskRecord);
        plannerTaskRecords.set(step.planner_task_id, taskRecord);
        if (!run.task_ids.includes(taskRecord.task_id)) {
          run.task_ids.push(taskRecord.task_id);
        }
      }
      taskRecord.step_ids = uniqueStrings([...taskRecord.step_ids, stepId]);
      taskRecord.updated_at = createdAt;
      taskRecord.status = taskStatusFromStepStatus(initialStatus);
      taskRecord.completed_at = undefined;
      taskRecord.metadata = {
        ...taskRecord.metadata,
        planner_task_id: step.planner_task_id,
        planner_step_ids: uniqueStrings([
          ...(Array.isArray(taskRecord.metadata?.planner_step_ids)
            ? taskRecord.metadata.planner_step_ids.filter((value): value is string => typeof value === 'string')
            : []),
          step.planner_step_id
        ]),
        phase: 'EXECUTING'
      };

      const record: WorkflowStepRecord = {
        step_id: stepId,
        run_id: runId,
        task_id: taskRecord.task_id,
        state_key: step.planner_step_id,
        title: step.title,
        sequence,
        status: initialStatus,
        attempt: 0,
        depends_on_step_ids: dependsOnStepIds,
        created_at: createdAt,
        updated_at: createdAt,
        metadata: {
          planner_task_id: step.planner_task_id,
          planner_step_id: step.planner_step_id,
          dependency_planner_step_ids: dependsOnPlannerSteps,
          phase: 'EXECUTING',
          skill_id: step.skill_id,
          operation: step.operation,
          ...(step.metadata ?? {})
        }
      };

      this.snapshot.steps.push(record);
      registered.push(record);
    }

    if (aggregatePhase) {
      aggregatePhase.depends_on_step_ids = steps
        .map((step) => internalStepIds.get(step.planner_step_id))
        .filter((stepId): stepId is string => Boolean(stepId));
      aggregatePhase.updated_at = createdAt;
      if (aggregatePhase.status === 'READY') {
        aggregatePhase.status = 'PENDING';
      }
    }

    for (const task of plannerTaskRecords.values()) {
      if (task.run_id === runId && task.metadata?.phase === 'EXECUTING') {
        this.recalculateTaskRecord(task, createdAt);
      }
    }

    run.metadata = {
      ...run.metadata,
      execution_plan_registered_at: createdAt,
      execution_plan_steps: steps.length,
      execution_plan_tasks: plannerTaskRecords.size
    };
    run.states = uniqueStrings([...run.states, ...steps.map((step) => step.planner_step_id)]);
    this.promoteReadySteps(runId, createdAt);
    this.recalculateRun(runId, createdAt);
    this.persist();
    return registered;
  }

  resumeRun(runId: string): WorkflowRunRecord {
    const run = this.requireRun(runId);
    if (run.status === 'COMPLETED' || run.status === 'FAILED' || run.status === 'DEAD_LETTER') {
      throw new Error('run_not_resumable');
    }

    const steps = this.listSteps(runId);
    const active = steps.find((step) => step.status === 'RUNNING');
    if (active) {
      return run;
    }

    this.promoteReadySteps(runId, nowIso());
    const refreshedSteps = this.listSteps(runId);
    const next = refreshedSteps.find((step) => step.status === 'READY') ?? refreshedSteps.find((step) => step.status === 'PENDING');
    if (!next) {
      throw new Error('run_has_no_pending_steps');
    }

    this.transitionStep({
      run_id: runId,
      step_id: next.step_id,
      status: 'RUNNING'
    });

    return this.requireRun(runId);
  }

  annotateRun(runId: string, metadata: Record<string, unknown>): WorkflowRunRecord {
    const run = this.requireRun(runId);
    const timestamp = nowIso();
    run.metadata = {
      ...run.metadata,
      ...metadata
    };
    run.updated_at = timestamp;
    this.persist();
    return run;
  }

  transitionPlannerStep(input: { run_id: string; planner_step_id: string; status: WorkflowStepStatus; artifacts?: WorkflowArtifact[]; error?: { code: string; message: string }; metadata?: Record<string, unknown> }): WorkflowStepRecord {
    const step = this.getPlannerStep(input.run_id, input.planner_step_id);
    if (!step) {
      throw new Error('planner_step_not_found');
    }

    return this.transitionStep({
      run_id: input.run_id,
      step_id: step.step_id,
      status: input.status,
      artifacts: input.artifacts,
      error: input.error,
      metadata: input.metadata
    });
  }

  cancelTask(runIdOrTaskId: string, taskIdOrReason?: string | { code: string; message: string }, maybeReason?: { code: string; message: string }): WorkflowTaskRecord {
    const runScopedTaskId = typeof taskIdOrReason === 'string' ? taskIdOrReason : undefined;
    const reason = (typeof taskIdOrReason === 'string' ? maybeReason : taskIdOrReason) ?? {
      code: 'TASK_CANCELLED',
      message: 'task was cancelled by operator request'
    };
    const task = runScopedTaskId ? this.getRunTask(runIdOrTaskId, runScopedTaskId) : this.getTask(runIdOrTaskId);
    if (!task) {
      throw new Error('task_not_found');
    }

    const stepIds = uniqueStrings(task.step_ids);
    if (stepIds.length === 0) {
      throw new Error('task_has_no_steps');
    }
    for (const stepId of stepIds) {
      const step = this.snapshot.steps.find((candidate) => candidate.run_id === task.run_id && candidate.step_id === stepId);
      if (!step || step.status === 'COMPLETED' || step.status === 'FAILED' || step.status === 'DEAD_LETTER') {
        continue;
      }
      this.transitionStep({
        run_id: task.run_id,
        step_id: stepId,
        status: 'DEAD_LETTER',
        error: reason
      });
    }

    return this.getRunTask(task.run_id, task.task_id) ?? this.getTask(task.task_id)!;
  }

  transitionStep(input: StepTransitionInput): WorkflowStepRecord {
    const run = this.requireRun(input.run_id);
    const step = this.snapshot.steps.find((candidate) => candidate.run_id === input.run_id && candidate.step_id === input.step_id);
    if (!step) {
      throw new Error('step_not_found');
    }

    this.assertValidTransition(step, input.status);

    const currentTimestamp = nowIso();
    const previousStatus = step.status;

    step.status = input.status;
    step.updated_at = currentTimestamp;
    step.metadata = {
      ...step.metadata,
      ...(input.metadata ?? {})
    };
    if (input.artifacts) {
      step.artifacts = input.artifacts;
    }
    if (input.error) {
      step.last_error = input.error;
    }

    if (input.status === 'RUNNING') {
      step.started_at = step.started_at ?? currentTimestamp;
      step.attempt = previousStatus === 'RUNNING' ? step.attempt : step.attempt + 1;
    }

    if (input.status === 'COMPLETED' || input.status === 'FAILED' || input.status === 'DEAD_LETTER') {
      step.completed_at = currentTimestamp;
    }

    const task = this.snapshot.tasks.find((candidate) => candidate.task_id === step.task_id);
    if (!task) {
      throw new Error('task_not_found');
    }

    this.recalculateTaskRecord(task, currentTimestamp);

    this.promoteReadySteps(run.run_id, currentTimestamp);
    this.recalculateRun(run.run_id, currentTimestamp);
    this.persist();
    return step;
  }

  private requireRun(runId: string): WorkflowRunRecord {
    const run = this.getRun(runId);
    if (!run) {
      throw new Error('run_not_found');
    }
    return run;
  }

  private createPlannerTaskRecord(
    runId: string,
    step: WorkflowExecutionStepBlueprint,
    sequence: number,
    createdAt: string
  ): WorkflowTaskRecord {
    return {
      task_id: `task_${runId}_${sanitizeIdentifier(step.planner_task_id)}`,
      run_id: runId,
      name: step.title,
      sequence,
      status: 'PENDING',
      step_ids: [],
      created_at: createdAt,
      updated_at: createdAt,
      completed_at: undefined,
      metadata: {
        planner_task_id: step.planner_task_id,
        planner_step_ids: [],
        phase: 'EXECUTING'
      }
    };
  }

  private findPlannerTaskRecord(runId: string, plannerTaskId: string): WorkflowTaskRecord | undefined {
    return this.snapshot.tasks.find(
      (task) => task.run_id === runId && task.metadata?.planner_task_id === plannerTaskId
    );
  }

  private assertValidTransition(step: WorkflowStepRecord, targetStatus: WorkflowStepStatus): void {
    if (step.status === targetStatus) {
      return;
    }
    if (step.status === 'COMPLETED' || step.status === 'FAILED' || step.status === 'DEAD_LETTER') {
      throw new Error('invalid_step_transition');
    }
    if (targetStatus === 'PENDING' || targetStatus === 'READY') {
      throw new Error('invalid_step_transition');
    }
    if (targetStatus !== 'DEAD_LETTER' && !this.areDependenciesResolved(step)) {
      throw new Error('step_dependencies_unresolved');
    }

    const allowedFrom: Record<NonTerminalWorkflowStepStatus, WorkflowStepStatus[]> = {
      PENDING: ['DEAD_LETTER'],
      READY: ['RUNNING', 'COMPLETED', 'FAILED', 'DEAD_LETTER'],
      RUNNING: ['COMPLETED', 'FAILED', 'DEAD_LETTER']
    };
    const currentStatus = step.status as NonTerminalWorkflowStepStatus;
    if (!allowedFrom[currentStatus].includes(targetStatus)) {
      throw new Error('invalid_step_transition');
    }
  }

  private areDependenciesResolved(step: WorkflowStepRecord): boolean {
    const dependencies = step.depends_on_step_ids ?? [];
    if (dependencies.length === 0) {
      return true;
    }
    const completedStepIds = new Set(
      this.snapshot.steps
        .filter((candidate) => candidate.run_id === step.run_id && candidate.status === 'COMPLETED')
        .map((candidate) => candidate.step_id)
    );
    return dependencies.every((dependency) => completedStepIds.has(dependency));
  }

  private promoteReadySteps(runId: string, timestamp: string): void {
    const completedStepIds = new Set(
      this.snapshot.steps
        .filter((step) => step.run_id === runId && step.status === 'COMPLETED')
        .map((step) => step.step_id)
    );

    for (const step of this.listSteps(runId)) {
      if (step.status !== 'PENDING') {
        continue;
      }
      const dependencies = step.depends_on_step_ids ?? [];
      if (dependencies.length === 0 || dependencies.every((dependency) => completedStepIds.has(dependency))) {
        step.status = 'READY';
        step.updated_at = timestamp;
        const task = this.snapshot.tasks.find((candidate) => candidate.task_id === step.task_id);
        if (task) {
          this.recalculateTaskRecord(task, timestamp);
        }
      }
    }
  }

  private recalculateTaskRecord(task: WorkflowTaskRecord, timestamp: string): void {
    const steps = task.step_ids
      .map((stepId) => this.snapshot.steps.find((candidate) => candidate.run_id === task.run_id && candidate.step_id === stepId))
      .filter((step): step is WorkflowStepRecord => Boolean(step));
    if (steps.some((step) => step.status === 'DEAD_LETTER')) {
      task.status = 'DEAD_LETTER';
      task.completed_at = timestamp;
    } else if (steps.some((step) => step.status === 'FAILED')) {
      task.status = 'FAILED';
      task.completed_at = timestamp;
    } else if (steps.length > 0 && steps.every((step) => step.status === 'COMPLETED')) {
      task.status = 'COMPLETED';
      task.completed_at = timestamp;
    } else if (steps.some((step) => step.status === 'RUNNING' || step.status === 'READY')) {
      task.status = 'RUNNING';
      task.completed_at = undefined;
    } else {
      task.status = 'PENDING';
      task.completed_at = undefined;
    }
    task.updated_at = timestamp;
  }

  private recalculateRun(runId: string, timestamp: string): void {
    const run = this.requireRun(runId);
    const orderedSteps = this.listSteps(runId);
    const deadLetterStep = orderedSteps.find((step) => step.status === 'DEAD_LETTER');
    if (deadLetterStep) {
      run.status = 'DEAD_LETTER';
      run.current_state = deadLetterStep.state_key;
      run.completed_at = timestamp;
      run.updated_at = timestamp;
      return;
    }

    const failedStep = orderedSteps.find((step) => step.status === 'FAILED');
    if (failedStep) {
      run.status = 'FAILED';
      run.current_state = failedStep.state_key;
      run.completed_at = timestamp;
      run.updated_at = timestamp;
      return;
    }

    if (orderedSteps.length > 0 && orderedSteps.every((step) => step.status === 'COMPLETED')) {
      run.status = 'COMPLETED';
      run.current_state = orderedSteps[orderedSteps.length - 1].state_key;
      run.completed_at = timestamp;
      run.updated_at = timestamp;
      return;
    }

    const activeStep =
      orderedSteps.find((step) => step.status === 'RUNNING') ??
      orderedSteps.find((step) => step.status === 'READY') ??
      orderedSteps.find((step) => step.status === 'PENDING');

    run.status = 'RUNNING';
    run.current_state = activeStep?.state_key ?? run.current_state;
    run.completed_at = undefined;
    run.updated_at = timestamp;
  }

  private loadSnapshot(): WorkflowSnapshot {
    if (!existsSync(this.filePath)) {
      return emptySnapshot();
    }

    try {
      const raw = readFileSync(this.filePath, 'utf8');
      const parsed = JSON.parse(raw) as Partial<WorkflowSnapshot>;
      if (!parsed || typeof parsed !== 'object') {
        return emptySnapshot();
      }
      return {
        version: 1,
        runs: Array.isArray(parsed.runs) ? (parsed.runs as WorkflowRunRecord[]) : [],
        tasks: Array.isArray(parsed.tasks) ? (parsed.tasks as WorkflowTaskRecord[]) : [],
        steps: Array.isArray(parsed.steps) ? (parsed.steps as WorkflowStepRecord[]) : []
      };
    } catch {
      return emptySnapshot();
    }
  }

  private persist(): void {
    mkdirSync(path.dirname(this.filePath), { recursive: true });
    const tmpPath = `${this.filePath}.${process.pid}.tmp`;
    writeFileSync(tmpPath, JSON.stringify(this.snapshot, null, 2));
    renameSync(tmpPath, this.filePath);
  }
}
