import { randomUUID } from 'node:crypto';
import { existsSync } from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

import {
  authenticateInternalRequest,
  resolveEffectiveUserScope
} from '../../../packages/shared/src/internal-http-auth.js';
import {
  loadBrevioEnvironment,
  resolveAccessTokenVerificationKey,
  requireSharedSecret
} from '../../../packages/shared/src/security.js';
import type {
  CreateWorkflowRunInput,
  WorkflowArtifact,
  WorkflowExecutionStepBlueprint,
  WorkflowStepBlueprint,
  WorkflowStepStatus
} from './workflow-store.js';
import { WorkflowStore } from './workflow-store.js';

type MessageWorkflowState =
  | 'RECEIVED'
  | 'CLASSIFYING'
  | 'DECOMPOSING'
  | 'EXECUTING'
  | 'AGGREGATING'
  | 'FORMATTING'
  | 'DELIVERING'
  | 'COMPLETED'
  | 'FAILED'
  | 'DEAD_LETTER';

type DailyWorkflowState = 'INIT' | 'COMPOSING' | 'DELIVERING' | 'COMPLETED';

interface WorkerConfig {
  serviceName: string;
  version: string;
  environment: string;
  port: number;
  shutdownTimeoutMs: number;
  maxBodyBytes: number;
  stateFilePath: string;
  internalAuthSecret: string;
  internalAuthIssuer: string;
  serviceAudience: string;
  callerContextSecret: string;
  logSalt: string;
}

interface RequestContext {
  traceId: string;
  spanId: string;
  requestId: string;
  subjectRef?: string;
}

interface WorkerRuntime {
  config: WorkerConfig;
  startedAtMs: number;
  server: http.Server;
  store: WorkflowStore;
  close(): Promise<void>;
}

interface A2ATaskPayload {
  task_id: string;
  run_id: string;
  name: string;
  status: string;
  created_at: string;
  updated_at: string;
  completed_at?: string;
  steps: ReturnType<WorkflowStore['listSteps']>;
  artifacts: WorkflowArtifact[];
  metadata: Record<string, unknown>;
}

function parsePositiveInt(raw: string | undefined, fallback: number, field: string): number {
  if (!raw || raw.trim() === '') {
    return fallback;
  }
  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`invalid ${field}: expected positive integer`);
  }
  return parsed;
}

function loadConfig(): WorkerConfig {
  const environment = loadBrevioEnvironment();
  return {
    serviceName: 'brevio-temporal-worker',
    version: process.env.SERVICE_VERSION ?? '0.2.0',
    environment,
    port: parsePositiveInt(process.env.PORT, 8087, 'PORT'),
    shutdownTimeoutMs: parsePositiveInt(process.env.BREVIO_TEMPORAL_WORKER_SHUTDOWN_TIMEOUT_MS, 30000, 'BREVIO_TEMPORAL_WORKER_SHUTDOWN_TIMEOUT_MS'),
    maxBodyBytes: parsePositiveInt(process.env.BREVIO_TEMPORAL_WORKER_MAX_BODY_BYTES, 256 * 1024, 'BREVIO_TEMPORAL_WORKER_MAX_BODY_BYTES'),
    stateFilePath:
      process.env.BREVIO_TEMPORAL_WORKER_STATE_FILE?.trim() ||
      path.join(process.cwd(), '.runtime', 'temporal-worker-state.json'),
    internalAuthSecret: resolveAccessTokenVerificationKey(
      process.env.BREVIO_INTERNAL_AUTH_PUBLIC_KEY,
      process.env.BREVIO_INTERNAL_AUTH_PRIVATE_KEY,
      process.env.BREVIO_INTERNAL_AUTH_SECRET,
      environment,
      'BREVIO_INTERNAL_AUTH_PUBLIC_KEY',
      'brevio-temporal-worker'
    ),
    internalAuthIssuer: process.env.BREVIO_INTERNAL_AUTH_ISSUER?.trim() || 'https://auth.brevio.internal',
    serviceAudience: process.env.BREVIO_TEMPORAL_WORKER_AUDIENCE?.trim() || 'brevio-temporal-worker',
    callerContextSecret: requireSharedSecret(process.env.BREVIO_CALLER_CONTEXT_SECRET, 'BREVIO_CALLER_CONTEXT_SECRET', environment, 'brevio-temporal-worker-caller'),
    logSalt: process.env.BREVIO_TEMPORAL_WORKER_LOG_SALT?.trim() || `brevio-temporal-worker:${environment}`
  };
}

function getHeader(req: http.IncomingMessage, name: string): string | undefined {
  const value = req.headers[name.toLowerCase()];
  if (typeof value === 'string') {
    return value;
  }
  if (Array.isArray(value) && value.length > 0) {
    return value[0];
  }
  return undefined;
}

function requestContext(req: http.IncomingMessage): RequestContext {
  return {
    traceId: getHeader(req, 'x-trace-id') ?? randomUUID(),
    spanId: getHeader(req, 'x-span-id') ?? randomUUID(),
    requestId: getHeader(req, 'x-request-id') ?? randomUUID(),
  };
}

function logEvent(
  runtime: WorkerRuntime,
  ctx: RequestContext,
  event: string,
  severity: 'INFO' | 'WARN' | 'ERROR',
  attrs: Record<string, unknown>
): void {
  process.stdout.write(
    JSON.stringify({
      ts: new Date().toISOString(),
      service: runtime.config.serviceName,
      env: runtime.config.environment,
      trace_id: ctx.traceId,
      span_id: ctx.spanId,
      request_id: ctx.requestId,
      subject_ref: ctx.subjectRef,
      event,
      severity,
      attrs
    }) + '\n'
  );
}

function sendJSON(res: http.ServerResponse, statusCode: number, payload: Record<string, unknown>): void {
  res.writeHead(statusCode, { 'content-type': 'application/json' });
  res.end(JSON.stringify(payload));
}

async function readRawBody(req: http.IncomingMessage, maxBytes: number): Promise<Buffer> {
  const chunks: Buffer[] = [];
  let bytes = 0;
  for await (const chunk of req) {
    const data = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    bytes += data.byteLength;
    if (bytes > maxBytes) {
      throw new Error('payload_too_large');
    }
    chunks.push(data);
  }
  return chunks.length > 0 ? Buffer.concat(chunks) : Buffer.from('{}', 'utf8');
}

function parseObject(rawBody: Buffer): Record<string, unknown> {
  try {
    const parsed = JSON.parse(rawBody.toString('utf8'));
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
    return {};
  } catch {
    throw new Error('invalid_json');
  }
}

function asString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function asObject(value: unknown): Record<string, unknown> | undefined {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return undefined;
}

function parseApiPath(pathname: string): string[] | undefined {
  const segments = pathname.split('/').filter((segment) => segment.length > 0);
  if (segments.length < 2) {
    return undefined;
  }
  if (segments[0] === 'api' && segments[1] === 'v1') {
    return segments.slice(2);
  }
  if (segments[0] === 'v1') {
    return segments.slice(1);
  }
  return undefined;
}

function fnv1a(input: string): number {
  let hash = 0x811c9dc5;
  for (let i = 0; i < input.length; i += 1) {
    hash ^= input.charCodeAt(i);
    hash +=
      (hash << 1) +
      (hash << 4) +
      (hash << 7) +
      (hash << 8) +
      (hash << 24);
  }
  return hash >>> 0;
}

function deterministicJitterMs(workflowRunId: string, attempt: number, maxJitterMs: number): number {
  if (maxJitterMs <= 0) {
    return 0;
  }
  return fnv1a(`${workflowRunId}:${attempt}`) % maxJitterMs;
}

function asArtifactArray(value: unknown): WorkflowArtifact[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }

  const artifacts = value
    .map((item) => {
      const artifact = asObject(item);
      const artifactId = asString(artifact?.artifact_id);
      const type = asString(artifact?.type);
      if (!artifactId || !type) {
        return undefined;
      }
      return {
        artifact_id: artifactId,
        type,
        uri: asString(artifact.uri),
        inline_data: artifact.inline_data
      } satisfies WorkflowArtifact;
    })
    .filter((artifact): artifact is WorkflowArtifact => Boolean(artifact));

  return artifacts.length > 0 ? artifacts : undefined;
}

function asStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }

  const values = value
    .map((item) => asString(item))
    .filter((item): item is string => Boolean(item));

  return values.length > 0 ? values : undefined;
}

function asExecutionPlanSteps(value: unknown): WorkflowExecutionStepBlueprint[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }

  const steps = value
    .map((item) => {
      const step = asObject(item);
      const plannerStepId = asString(step?.planner_step_id);
      const plannerTaskId = asString(step?.planner_task_id);
      const title = asString(step?.title);
      if (!plannerStepId || !plannerTaskId || !title) {
        return undefined;
      }
      return {
        planner_step_id: plannerStepId,
        planner_task_id: plannerTaskId,
        title,
        skill_id: asString(step.skill_id),
        operation: asString(step.operation),
        dependencies: asStringArray(step.dependencies),
        metadata: asObject(step.metadata)
      } satisfies WorkflowExecutionStepBlueprint;
    })
    .filter((step): step is WorkflowExecutionStepBlueprint => Boolean(step));

  return steps.length > 0 ? steps : undefined;
}

function baseMessageStates(): MessageWorkflowState[] {
  return ['RECEIVED', 'CLASSIFYING', 'DECOMPOSING', 'EXECUTING', 'AGGREGATING', 'FORMATTING', 'DELIVERING', 'COMPLETED'];
}

function baseDailyStates(): DailyWorkflowState[] {
  return ['INIT', 'COMPOSING', 'DELIVERING', 'COMPLETED'];
}

function buildCompletedBlueprints<TState extends string>(states: TState[]): WorkflowStepBlueprint[] {
  return states.map((state) => ({
    state_key: state,
    title: state,
    status: 'COMPLETED'
  }));
}

function buildPausedBlueprints<TState extends string>(states: TState[], pauseAfterState: TState): WorkflowStepBlueprint[] {
  const pauseIndex = states.indexOf(pauseAfterState);
  if (pauseIndex < 0 || pauseAfterState === states[states.length - 1]) {
    return buildCompletedBlueprints(states);
  }

  return states.map((state, index) => ({
    state_key: state,
    title: state,
    status: index < pauseIndex ? 'COMPLETED' : index === pauseIndex ? 'RUNNING' : 'PENDING'
  }));
}

function buildMessageWorkflowPlan(
  failureState?: MessageWorkflowState,
  pauseAfterState?: MessageWorkflowState
): Pick<CreateWorkflowRunInput, 'status' | 'current_state' | 'completed_at' | 'steps'> {
  const ordered = baseMessageStates();

  if (failureState && failureState !== 'COMPLETED') {
    const idx = ordered.indexOf(failureState);
    const workflowSteps = idx >= 0 ? ordered.slice(0, idx + 1) : ordered.slice(0, ordered.length - 1);
    const terminalState = failureState === 'RECEIVED' ? 'DEAD_LETTER' : 'FAILED';
    const steps = [
      ...workflowSteps.map((state) => ({
        state_key: state,
        title: state,
        status: 'COMPLETED' as WorkflowStepStatus
      })),
      {
        state_key: terminalState,
        title: terminalState,
        status: terminalState
      }
    ];

    if (idx >= 0 && steps.length > 1) {
      steps[steps.length - 2].status = 'COMPLETED';
    }

    return {
      status: terminalState,
      current_state: terminalState,
      completed_at: new Date().toISOString(),
      steps: steps.map((step, index) =>
        index === steps.length - 1
          ? { ...step, status: terminalState }
          : step
      )
    };
  }

  if (pauseAfterState) {
    const steps = buildPausedBlueprints(ordered, pauseAfterState);
    if (steps.some((step) => step.status !== 'COMPLETED')) {
      return {
        status: 'RUNNING',
        current_state: pauseAfterState,
        completed_at: undefined,
        steps
      };
    }
  }

  return {
    status: 'COMPLETED',
    current_state: 'COMPLETED',
    completed_at: new Date().toISOString(),
    steps: buildCompletedBlueprints(ordered)
  };
}

function buildDailyWorkflowPlan(
  pauseAfterState?: DailyWorkflowState
): Pick<CreateWorkflowRunInput, 'status' | 'current_state' | 'completed_at' | 'steps'> {
  const ordered = baseDailyStates();
  if (pauseAfterState) {
    const steps = buildPausedBlueprints(ordered, pauseAfterState);
    if (steps.some((step) => step.status !== 'COMPLETED')) {
      return {
        status: 'RUNNING',
        current_state: pauseAfterState,
        completed_at: undefined,
        steps
      };
    }
  }

  return {
    status: 'COMPLETED',
    current_state: 'COMPLETED',
    completed_at: new Date().toISOString(),
    steps: buildCompletedBlueprints(ordered)
  };
}

function parseStepStatus(value: unknown): WorkflowStepStatus | undefined {
  const normalized = asString(value);
  switch (normalized) {
    case 'PENDING':
    case 'READY':
    case 'RUNNING':
    case 'COMPLETED':
    case 'FAILED':
    case 'DEAD_LETTER':
      return normalized;
    default:
      return undefined;
  }
}

function healthPayload(runtime: WorkerRuntime, deep: boolean): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    status: 'healthy',
    version: runtime.config.version,
    uptime_ms: Date.now() - runtime.startedAtMs
  };

  if (!deep) {
    return payload;
  }

  return {
    ...payload,
    checks: {
      process: 'ok',
      workflow_store_mode: 'local_file_snapshot',
      durable_execution: false,
      state_file_exists: existsSync(runtime.config.stateFilePath)
    },
    workflow_store: runtime.store.stats()
  };
}

function agentCardPayload(runtime: WorkerRuntime): Record<string, unknown> {
  return {
    agent_id: runtime.config.serviceName,
    name: 'Brevio Temporal Worker',
    description: 'Durable workflow runtime for Brevio A2A task lifecycle, task queries, and execution-plan tracking.',
    version: runtime.config.version,
    protocol_version: '2026.a2a.v1',
    default_endpoint: `http://localhost:${runtime.config.port}/api/v1/a2a`,
    capabilities: [
      {
        id: 'task.lifecycle',
        name: 'Task lifecycle',
        description: 'Create, query, and cancel durable workflow tasks.',
        version: '1.0.0',
        input_modes: ['application/json'],
        output_modes: ['application/json'],
        async: true
      },
      {
        id: 'artifact.updates',
        name: 'Artifact updates',
        description: 'Expose artifacts emitted by workflow steps and planned execution steps.',
        version: '1.0.0',
        input_modes: ['application/json'],
        output_modes: ['application/json'],
        async: true
      }
    ],
    supports: {
      task_lifecycle: true,
      task_query: true,
      artifact_updates: true,
      push_callbacks: false,
      capability_inventory: false
    }
  };
}

function serializeA2ATask(runtime: WorkerRuntime, taskId: string, runId?: string): A2ATaskPayload | undefined {
  const task = runId ? runtime.store.getRunTask(runId, taskId) : runtime.store.getTask(taskId);
  if (!task) {
    return undefined;
  }

  const stepIds = new Set(task.step_ids);
  const steps = runtime.store.listSteps(task.run_id).filter((step) => stepIds.has(step.step_id));
  const artifacts = steps.flatMap((step) => step.artifacts ?? []);

  return {
    task_id: task.task_id,
    run_id: task.run_id,
    name: task.name,
    status: task.status,
    created_at: task.created_at,
    updated_at: task.updated_at,
    completed_at: task.completed_at,
    steps,
    artifacts,
    metadata: task.metadata
  };
}

function buildRuntime(config?: WorkerConfig): WorkerRuntime {
  const resolvedConfig = config ?? loadConfig();
  const startedAtMs = Date.now();
  const store = new WorkflowStore(resolvedConfig.stateFilePath);

  let runtimeRef: WorkerRuntime | undefined;

  const server = http.createServer((req, res) => {
    const runtime = runtimeRef;
    if (!runtime) {
      sendJSON(res, 500, { error: 'runtime_not_ready' });
      return;
    }

    const ctx = requestContext(req);
    const method = req.method ?? 'GET';
    const pathname = new URL(req.url ?? '/', 'http://localhost').pathname;

    const onError = (statusCode: number, code: string): void => {
      sendJSON(res, statusCode, { error: code });
      logEvent(runtime, ctx, 'temporal_worker.request.error', 'WARN', {
        method,
        path: pathname,
        status_code: statusCode,
        code
      });
    };

    if (method === 'GET' && pathname === '/health') {
      sendJSON(res, 200, healthPayload(runtime, false));
      return;
    }

    if (method === 'GET' && pathname === '/.well-known/agent-card.json') {
      sendJSON(res, 200, agentCardPayload(runtime));
      return;
    }

    if (method === 'GET' && pathname === '/health/deep') {
      try {
        authenticateInternalRequest(req, runtime.config, ctx, { mode: 'admin' });
      } catch (error) {
        onError(401, error instanceof Error ? error.message : 'admin_token_required');
        return;
      }
      sendJSON(res, 200, healthPayload(runtime, true));
      return;
    }

    const segments = parseApiPath(pathname);
    if (!segments || (segments[0] !== 'temporal-worker' && segments[0] !== 'a2a')) {
      onError(404, 'not_found');
      return;
    }

    const isWorkflowStartRoute =
      method === 'POST' &&
      segments[0] === 'temporal-worker' &&
      segments.length === 3 &&
      segments[1] === 'workflows' &&
      (segments[2] === 'message-processing' || segments[2] === 'daily-rhythm');
    let auth: ReturnType<typeof authenticateInternalRequest>;
    try {
      auth = authenticateInternalRequest(req, runtime.config, ctx, {
        allowedTokenUses: isWorkflowStartRoute
          ? ['service_access', 'admin_access', 'user_access']
          : ['service_access', 'admin_access']
      });
    } catch (error) {
      onError(401, error instanceof Error ? error.message : 'authorization_required');
      return;
    }

    if (method === 'GET' && segments.length === 2 && segments[0] === 'a2a' && segments[1] === 'tasks') {
      const runId = new URL(req.url ?? '/', 'http://localhost').searchParams.get('run_id')?.trim();
      const limitRaw = Number(new URL(req.url ?? '/', 'http://localhost').searchParams.get('limit') ?? '100');
      const limit = Number.isFinite(limitRaw) && limitRaw > 0 ? Math.floor(limitRaw) : 100;
      const tasks = (runId ? runtime.store.listTasks(runId) : runtime.store.listRuns().flatMap((run) => runtime.store.listTasks(run.run_id)))
        .slice(0, limit)
        .map((task) => serializeA2ATask(runtime, task.task_id))
        .filter((task): task is A2ATaskPayload => Boolean(task));
      sendJSON(res, 200, {
        total: tasks.length,
        tasks
      });
      return;
    }

    if (method === 'GET' && segments.length === 3 && segments[0] === 'a2a' && segments[1] === 'tasks') {
      const runId = new URL(req.url ?? '/', 'http://localhost').searchParams.get('run_id')?.trim();
      if (!runId) {
        onError(400, 'run_id_required');
        return;
      }
      const task = serializeA2ATask(runtime, segments[2], runId);
      if (!task) {
        onError(404, 'task_not_found');
        return;
      }
      sendJSON(res, 200, task as unknown as Record<string, unknown>);
      return;
    }

    if (method === 'POST' && segments.length === 4 && segments[0] === 'a2a' && segments[1] === 'tasks' && segments[3] === 'cancel') {
      void (async () => {
        const runId = new URL(req.url ?? '/', 'http://localhost').searchParams.get('run_id')?.trim();
        if (!runId) {
          onError(400, 'run_id_required');
          return;
        }
        try {
          const task = runtime.store.cancelTask(runId, segments[2]);
          const serialized = serializeA2ATask(runtime, task.task_id, runId);
          sendJSON(res, 200, serialized as unknown as Record<string, unknown>);
          logEvent(runtime, ctx, 'temporal_worker.a2a.task.cancelled', 'INFO', {
            task_id: task.task_id,
            run_id: task.run_id
          });
        } catch (err) {
          const code = err instanceof Error ? err.message : 'task_cancel_failed';
          if (code === 'task_not_found') {
            onError(404, code);
            return;
          }
          if (code === 'invalid_step_transition' || code === 'step_dependencies_unresolved') {
            onError(409, code);
            return;
          }
          onError(500, 'task_cancel_failed');
        }
      })();
      return;
    }

    if (
      method === 'GET' &&
      segments.length === 5 &&
      segments[0] === 'a2a' &&
      segments[1] === 'runs' &&
      segments[3] === 'tasks'
    ) {
      if (!runtime.store.getRun(segments[2])) {
        onError(404, 'run_not_found');
        return;
      }
      const task = serializeA2ATask(runtime, segments[4], segments[2]);
      if (!task) {
        onError(404, 'task_not_found');
        return;
      }
      sendJSON(res, 200, task as unknown as Record<string, unknown>);
      return;
    }

    if (
      method === 'POST' &&
      segments.length === 6 &&
      segments[0] === 'a2a' &&
      segments[1] === 'runs' &&
      segments[3] === 'tasks' &&
      segments[5] === 'cancel'
    ) {
      void (async () => {
        if (!runtime.store.getRun(segments[2])) {
          onError(404, 'run_not_found');
          return;
        }
        try {
          const task = runtime.store.cancelTask(segments[2], segments[4]);
          const serialized = serializeA2ATask(runtime, task.task_id, segments[2]);
          sendJSON(res, 200, serialized as unknown as Record<string, unknown>);
          logEvent(runtime, ctx, 'temporal_worker.a2a.run_task.cancelled', 'INFO', {
            task_id: task.task_id,
            run_id: task.run_id
          });
        } catch (err) {
          const code = err instanceof Error ? err.message : 'task_cancel_failed';
          if (code === 'run_not_found' || code === 'task_not_found') {
            onError(404, code);
            return;
          }
          if (code === 'invalid_step_transition' || code === 'step_dependencies_unresolved') {
            onError(409, code);
            return;
          }
          onError(500, 'task_cancel_failed');
        }
      })();
      return;
    }

    if (segments[0] === 'a2a') {
      onError(404, 'not_found');
      return;
    }

    if (method === 'GET' && segments.length === 2 && segments[1] === 'workflows') {
      sendJSON(res, 200, {
        workflows: [
          {
            id: 'message-processing',
            task_queue: 'message-processing',
            execution_timeout_seconds: 120,
            states: [
              'RECEIVED',
              'CLASSIFYING',
              'DECOMPOSING',
              'EXECUTING',
              'AGGREGATING',
              'FORMATTING',
              'DELIVERING',
              'COMPLETED',
              'FAILED',
              'DEAD_LETTER'
            ]
          },
          {
            id: 'daily-rhythm',
            task_queue: 'message-processing',
            states: ['INIT', 'COMPOSING', 'DELIVERING', 'COMPLETED']
          }
        ]
      });
      return;
    }

    if (method === 'GET' && segments.length === 2 && segments[1] === 'runs') {
      sendJSON(res, 200, {
        total: runtime.store.listRuns().length,
        runs: runtime.store.listRuns()
      });
      return;
    }

    if (method === 'GET' && segments.length === 3 && segments[1] === 'runs') {
      const run = runtime.store.getRun(segments[2]);
      if (!run) {
        onError(404, 'run_not_found');
        return;
      }
      sendJSON(res, 200, run as unknown as Record<string, unknown>);
      return;
    }

    if (method === 'GET' && segments.length === 4 && segments[1] === 'runs' && segments[3] === 'tasks') {
      if (!runtime.store.getRun(segments[2])) {
        onError(404, 'run_not_found');
        return;
      }
      sendJSON(res, 200, {
        run_id: segments[2],
        total: runtime.store.listTasks(segments[2]).length,
        tasks: runtime.store.listTasks(segments[2])
      });
      return;
    }

    if (method === 'POST' && segments.length === 4 && segments[1] === 'runs' && segments[3] === 'metadata') {
      void (async () => {
        const rawBody = await readRawBody(req, runtime.config.maxBodyBytes);
        const payload = parseObject(rawBody);
        const metadata = asObject(payload.metadata) ?? {};
        try {
          const run = runtime.store.annotateRun(segments[2], metadata);
          sendJSON(res, 200, run as unknown as Record<string, unknown>);
          logEvent(runtime, ctx, 'temporal_worker.run.annotated', 'INFO', {
            run_id: segments[2],
            metadata_keys: Object.keys(metadata)
          });
        } catch (err) {
          const code = err instanceof Error ? err.message : 'run_annotation_failed';
          if (code === 'run_not_found') {
            onError(404, code);
            return;
          }
          onError(500, 'run_annotation_failed');
        }
      })().catch((err) => {
        const code = err instanceof Error ? err.message : 'run_annotation_failed';
        if (code === 'payload_too_large') {
          onError(413, code);
          return;
        }
        if (code === 'invalid_json') {
          onError(400, code);
          return;
        }
        onError(500, 'run_annotation_failed');
      });
      return;
    }

    if (method === 'GET' && segments.length === 4 && segments[1] === 'runs' && segments[3] === 'steps') {
      if (!runtime.store.getRun(segments[2])) {
        onError(404, 'run_not_found');
        return;
      }
      sendJSON(res, 200, {
        run_id: segments[2],
        total: runtime.store.listSteps(segments[2]).length,
        steps: runtime.store.listSteps(segments[2])
      });
      return;
    }

    if (method === 'GET' && segments.length === 5 && segments[1] === 'runs' && segments[3] === 'planner-steps') {
      if (!runtime.store.getRun(segments[2])) {
        onError(404, 'run_not_found');
        return;
      }
      const step = runtime.store.getPlannerStep(segments[2], segments[4]);
      if (!step) {
        onError(404, 'planner_step_not_found');
        return;
      }
      sendJSON(res, 200, step as unknown as Record<string, unknown>);
      return;
    }

    if (method === 'POST' && segments.length === 4 && segments[1] === 'runs' && segments[3] === 'resume') {
      void (async () => {
        try {
          const run = runtime.store.resumeRun(segments[2]);
          sendJSON(res, 200, run as unknown as Record<string, unknown>);
          logEvent(runtime, ctx, 'temporal_worker.run.resumed', 'INFO', {
            run_id: run.run_id,
            current_state: run.current_state,
            status: run.status
          });
        } catch (err) {
          const code = err instanceof Error ? err.message : 'resume_failed';
          if (code === 'run_not_found') {
            onError(404, code);
            return;
          }
          if (code === 'run_not_resumable' || code === 'run_has_no_pending_steps') {
            onError(409, code);
            return;
          }
          onError(500, 'resume_failed');
        }
      })();
      return;
    }

    if (method === 'POST' && segments.length === 6 && segments[1] === 'runs' && segments[3] === 'steps' && segments[5] === 'transition') {
      void (async () => {
        const rawBody = await readRawBody(req, runtime.config.maxBodyBytes);
        const payload = parseObject(rawBody);
        const status = parseStepStatus(payload.status);
        if (!status) {
          onError(400, 'step_status_required');
          return;
        }

        const errorPayload = asObject(payload.error);

        try {
          const step = runtime.store.transitionStep({
            run_id: segments[2],
            step_id: segments[4],
            status,
            artifacts: asArtifactArray(payload.artifacts),
            error:
              errorPayload && asString(errorPayload.code) && asString(errorPayload.message)
                ? {
                    code: asString(errorPayload.code)!,
                    message: asString(errorPayload.message)!
                  }
                : undefined,
            metadata: asObject(payload.metadata)
          });
          sendJSON(res, 200, step as unknown as Record<string, unknown>);
          logEvent(runtime, ctx, 'temporal_worker.step.transitioned', 'INFO', {
            run_id: segments[2],
            step_id: segments[4],
            status
          });
        } catch (err) {
          const code = err instanceof Error ? err.message : 'step_transition_failed';
          if (code === 'run_not_found' || code === 'step_not_found') {
            onError(404, code);
            return;
          }
          if (code === 'invalid_step_transition' || code === 'step_dependencies_unresolved') {
            onError(409, code);
            return;
          }
          onError(500, 'step_transition_failed');
          logEvent(runtime, ctx, 'temporal_worker.step.transition_failed', 'ERROR', {
            run_id: segments[2],
            step_id: segments[4],
            message: err instanceof Error ? err.message : String(err)
          });
        }
      })().catch((err) => {
        const code = err instanceof Error ? err.message : 'step_transition_failed';
        if (code === 'payload_too_large') {
          onError(413, code);
          return;
        }
        if (code === 'invalid_json') {
          onError(400, code);
          return;
        }
        onError(500, 'step_transition_failed');
      });
      return;
    }

    if (method === 'POST' && segments.length === 6 && segments[1] === 'runs' && segments[3] === 'planner-steps' && segments[5] === 'transition') {
      void (async () => {
        const rawBody = await readRawBody(req, runtime.config.maxBodyBytes);
        const payload = parseObject(rawBody);
        const status = parseStepStatus(payload.status);
        if (!status) {
          onError(400, 'step_status_required');
          return;
        }

        const errorPayload = asObject(payload.error);

        try {
          const step = runtime.store.transitionPlannerStep({
            run_id: segments[2],
            planner_step_id: segments[4],
            status,
            artifacts: asArtifactArray(payload.artifacts),
            error:
              errorPayload && asString(errorPayload.code) && asString(errorPayload.message)
                ? {
                    code: asString(errorPayload.code)!,
                    message: asString(errorPayload.message)!
                  }
                : undefined,
            metadata: asObject(payload.metadata)
          });
          sendJSON(res, 200, step as unknown as Record<string, unknown>);
          logEvent(runtime, ctx, 'temporal_worker.planner_step.transitioned', 'INFO', {
            run_id: segments[2],
            planner_step_id: segments[4],
            status
          });
        } catch (err) {
          const code = err instanceof Error ? err.message : 'planner_step_transition_failed';
          if (code === 'run_not_found' || code === 'planner_step_not_found') {
            onError(404, code);
            return;
          }
          if (code === 'invalid_step_transition' || code === 'step_dependencies_unresolved') {
            onError(409, code);
            return;
          }
          onError(500, 'planner_step_transition_failed');
          logEvent(runtime, ctx, 'temporal_worker.planner_step.transition_failed', 'ERROR', {
            run_id: segments[2],
            planner_step_id: segments[4],
            message: err instanceof Error ? err.message : String(err)
          });
        }
      })().catch((err) => {
        const code = err instanceof Error ? err.message : 'planner_step_transition_failed';
        if (code === 'payload_too_large') {
          onError(413, code);
          return;
        }
        if (code === 'invalid_json') {
          onError(400, code);
          return;
        }
        onError(500, 'planner_step_transition_failed');
      });
      return;
    }

    if (method === 'POST' && segments.length === 4 && segments[1] === 'runs' && segments[3] === 'execution-plan') {
      void (async () => {
        const rawBody = await readRawBody(req, runtime.config.maxBodyBytes);
        const payload = parseObject(rawBody);
        const steps = asExecutionPlanSteps(payload.steps);
        if (!steps || steps.length === 0) {
          onError(400, 'execution_plan_steps_required');
          return;
        }

        try {
          const registered = runtime.store.registerExecutionPlan(segments[2], steps);
          sendJSON(res, 200, {
            run_id: segments[2],
            total: registered.length,
            steps: registered
          });
          logEvent(runtime, ctx, 'temporal_worker.execution_plan.registered', 'INFO', {
            run_id: segments[2],
            steps: registered.length
          });
        } catch (err) {
          const code = err instanceof Error ? err.message : 'execution_plan_register_failed';
          if (code === 'run_not_found' || code === 'executing_phase_not_found') {
            onError(404, code);
            return;
          }
          onError(500, 'execution_plan_register_failed');
        }
      })().catch((err) => {
        const code = err instanceof Error ? err.message : 'execution_plan_register_failed';
        if (code === 'payload_too_large') {
          onError(413, code);
          return;
        }
        if (code === 'invalid_json') {
          onError(400, code);
          return;
        }
        onError(500, 'execution_plan_register_failed');
      });
      return;
    }

    if (method === 'POST' && segments.length === 3 && segments[1] === 'workflows' && segments[2] === 'message-processing') {
      void (async () => {
        const rawBody = await readRawBody(req, runtime.config.maxBodyBytes);
        const payload = parseObject(rawBody);
        const scope = resolveEffectiveUserScope(auth, { requireUserId: true });

        const messageId = asString(payload.message_id);
        if (!messageId) {
          onError(400, 'message_id_required');
          return;
        }

        const workflowId = `msg-${messageId}`;
        const failState = asString(payload.force_fail_state) as MessageWorkflowState | undefined;
        const now = new Date().toISOString();
        const pauseAfterState = asString(payload.pause_after_state) as MessageWorkflowState | undefined;
        const plan = buildMessageWorkflowPlan(failState, pauseAfterState);
        const runId = randomUUID();
        const run = runtime.store.createRun({
          run_id: runId,
          workflow_id: workflowId,
          workflow_type: 'message-processing',
          user_id: scope.userId,
          status: plan.status,
          current_state: plan.current_state,
          started_at: now,
          completed_at: plan.completed_at,
          metadata: {
            message_id: messageId,
            deterministic_jitter_ms: deterministicJitterMs(runId, 1, 500),
            pause_after_state: pauseAfterState
          },
          steps: plan.steps
        });

        sendJSON(res, 202, run as unknown as Record<string, unknown>);
        logEvent(runtime, ctx, 'temporal_worker.message_processing.started', 'INFO', {
          workflow_id: workflowId,
          run_id: run.run_id,
          status: run.status,
          current_state: run.current_state
        });
      })().catch((err) => {
        const code = err instanceof Error ? err.message : 'workflow_start_failed';
        if (code === 'payload_too_large') {
          onError(413, code);
          return;
        }
        if (code === 'invalid_json') {
          onError(400, code);
          return;
        }
        if (code === 'caller_context_required') {
          onError(400, code);
          return;
        }
        onError(500, 'workflow_start_failed');
        logEvent(runtime, ctx, 'temporal_worker.message_processing.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'POST' && segments.length === 3 && segments[1] === 'workflows' && segments[2] === 'daily-rhythm') {
      void (async () => {
        const rawBody = await readRawBody(req, runtime.config.maxBodyBytes);
        const payload = parseObject(rawBody);
        const scope = resolveEffectiveUserScope(auth, { requireUserId: true });
        const userId = scope.userId!;

        const workflowId = `daily-rhythm-${userId}-${new Date().toISOString().slice(0, 10)}`;
        const now = new Date().toISOString();
        const pauseAfterState = asString(payload.pause_after_state) as DailyWorkflowState | undefined;
        const plan = buildDailyWorkflowPlan(pauseAfterState);
        const runId = randomUUID();
        const run = runtime.store.createRun({
          run_id: runId,
          workflow_id: workflowId,
          workflow_type: 'daily-rhythm',
          user_id: userId,
          status: plan.status,
          current_state: plan.current_state,
          started_at: now,
          completed_at: plan.completed_at,
          metadata: {
            wake_time: asString(payload.wake_time) ?? '07:00',
            deterministic_jitter_ms: deterministicJitterMs(runId, 1, 1000),
            pause_after_state: pauseAfterState
          },
          steps: plan.steps
        });

        sendJSON(res, 202, run as unknown as Record<string, unknown>);
        logEvent(runtime, ctx, 'temporal_worker.daily_rhythm.started', 'INFO', {
          workflow_id: workflowId,
          run_id: run.run_id,
          status: run.status,
          current_state: run.current_state
        });
      })().catch((err) => {
        const code = err instanceof Error ? err.message : 'workflow_start_failed';
        if (code === 'payload_too_large') {
          onError(413, code);
          return;
        }
        if (code === 'invalid_json') {
          onError(400, code);
          return;
        }
        if (code === 'caller_context_required') {
          onError(400, code);
          return;
        }
        onError(500, 'workflow_start_failed');
        logEvent(runtime, ctx, 'temporal_worker.daily_rhythm.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    onError(404, 'not_found');
  });

  const runtime: WorkerRuntime = {
    config: resolvedConfig,
    startedAtMs,
    server,
    store,
    async close(): Promise<void> {
      await new Promise<void>((resolve, reject) => {
        server.close((err) => {
          if (err) {
            reject(err);
            return;
          }
          resolve();
        });
      });
    }
  };

  runtimeRef = runtime;
  return runtime;
}

function installSignalHandlers(runtime: WorkerRuntime): void {
  const shutdown = async (signal: string): Promise<void> => {
    const ctx: RequestContext = {
      traceId: randomUUID(),
      spanId: randomUUID(),
      requestId: randomUUID()
    };

    logEvent(runtime, ctx, 'temporal_worker.shutdown.start', 'INFO', { signal });

    const timeout = setTimeout(() => {
      logEvent(runtime, ctx, 'temporal_worker.shutdown.timeout', 'ERROR', {
        timeout_ms: runtime.config.shutdownTimeoutMs
      });
      process.exit(1);
    }, runtime.config.shutdownTimeoutMs);

    try {
      await runtime.close();
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'temporal_worker.shutdown.complete', 'INFO', {});
      process.exit(0);
    } catch (err) {
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'temporal_worker.shutdown.failed', 'ERROR', {
        message: err instanceof Error ? err.message : String(err)
      });
      process.exit(1);
    }
  };

  process.on('SIGTERM', () => {
    void shutdown('SIGTERM');
  });
  process.on('SIGINT', () => {
    void shutdown('SIGINT');
  });
}

async function main(): Promise<void> {
  const runtime = buildRuntime();

  await new Promise<void>((resolve, reject) => {
    runtime.server.listen(runtime.config.port, () => resolve());
    runtime.server.once('error', (err) => reject(err));
  });

  installSignalHandlers(runtime);

  const ctx: RequestContext = {
    traceId: randomUUID(),
    spanId: randomUUID(),
    requestId: randomUUID()
  };

  logEvent(runtime, ctx, 'temporal_worker.started', 'INFO', {
    port: runtime.config.port,
    workflows: ['message-processing', 'daily-rhythm'],
    state_file_path: runtime.config.stateFilePath
  });
}

if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url) {
  void main().catch((err) => {
    process.stderr.write(
      JSON.stringify({
        ts: new Date().toISOString(),
        service: 'brevio-temporal-worker',
        event: 'temporal_worker.start.failed',
        severity: 'ERROR',
        message: err instanceof Error ? err.message : String(err)
      }) + '\n'
    );
    process.exit(1);
  });
}

export { buildRuntime as createTemporalWorkerRuntime };
