import { randomUUID } from 'node:crypto';
import http from 'node:http';
import { pathToFileURL } from 'node:url';

import { parseCapabilityInventory, resolveCapabilityInventory } from '../../../packages/shared/src/capability-inventory.js';
import type {
  CacheClient,
  SkillContext,
  SkillInput,
  SkillResult,
  StructuredLogger,
  Tracer,
  UserProfile
} from '@brevio/shared';

import { evaluateApprovalGate, type ExecutionPolicy } from './approval-policy.js';
import { applyExecutionRefs, parseExecutionRefs, type ExecutionRefs } from './execution-refs.js';
import { isHandsExecutableAdapter } from './skills/plane-policy.js';
import { getSkillAdapter, SkillRegistry } from './skills/index.js';
import { reportExecutionResult } from './workflow-runtime.js';

type CircuitBreakerState = 'CLOSED' | 'HALF_OPEN' | 'OPEN';

interface CircuitStateEntry {
  state: CircuitBreakerState;
  failureCount: number;
  openedAtMs?: number;
  halfOpenRemaining: number;
  updatedAtMs: number;
}

interface HandsConfig {
  serviceName: string;
  version: string;
  environment: string;
  port: number;
  shutdownTimeoutMs: number;
  executionTimeoutMs: number;
  maxBodyBytes: number;
  circuitFailureThreshold: number;
  circuitRecoveryTimeoutMs: number;
  circuitHalfOpenMaxCalls: number;
  temporalWorkerBaseUrl?: string;
  temporalWorkerTimeoutMs: number;
  capabilityInventoryJson?: string;
}

interface RequestContext {
  traceId: string;
  spanId: string;
  requestId: string;
  userId?: string;
}

interface ExecuteRequest {
  skill_id: string;
  request_id?: string;
  run_id?: string;
  task_id?: string;
  step_id?: string;
  attempt?: number;
  user_id?: string;
  tenant_id?: string;
  workspace_id?: string;
  allowed_skills?: string[];
  input: SkillInput;
  user_profile?: Partial<UserProfile>;
  config?: Record<string, unknown>;
  policy?: ExecutionPolicy;
}

interface HandsRuntime {
  config: HandsConfig;
  startedAtMs: number;
  server: http.Server;
  skills: string[];
  circuits: Map<string, CircuitStateEntry>;
  close(): Promise<void>;
}

class SkillExecutionTimeoutError extends Error {
  constructor(skillId: string, timeoutMs: number) {
    super(`skill ${skillId} timed out after ${timeoutMs}ms`);
    this.name = 'SkillExecutionTimeoutError';
  }
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

function loadConfig(): HandsConfig {
  return {
    serviceName: 'brevio-hands',
    version: process.env.SERVICE_VERSION ?? '0.2.0',
    environment: process.env.NODE_ENV ?? 'development',
    port: parsePositiveInt(process.env.PORT, 8082, 'PORT'),
    shutdownTimeoutMs: parsePositiveInt(process.env.BREVIO_HANDS_SHUTDOWN_TIMEOUT_MS, 30000, 'BREVIO_HANDS_SHUTDOWN_TIMEOUT_MS'),
    executionTimeoutMs: parsePositiveInt(process.env.BREVIO_HANDS_EXECUTION_TIMEOUT_MS, 60000, 'BREVIO_HANDS_EXECUTION_TIMEOUT_MS'),
    maxBodyBytes: parsePositiveInt(process.env.BREVIO_HANDS_MAX_BODY_BYTES, 2 * 1024 * 1024, 'BREVIO_HANDS_MAX_BODY_BYTES'),
    circuitFailureThreshold: parsePositiveInt(process.env.BREVIO_HANDS_CB_FAILURE_THRESHOLD, 5, 'BREVIO_HANDS_CB_FAILURE_THRESHOLD'),
    circuitRecoveryTimeoutMs: parsePositiveInt(process.env.BREVIO_HANDS_CB_RECOVERY_TIMEOUT_MS, 60000, 'BREVIO_HANDS_CB_RECOVERY_TIMEOUT_MS'),
    circuitHalfOpenMaxCalls: parsePositiveInt(process.env.BREVIO_HANDS_CB_HALF_OPEN_MAX_CALLS, 3, 'BREVIO_HANDS_CB_HALF_OPEN_MAX_CALLS'),
    temporalWorkerBaseUrl: process.env.BREVIO_TEMPORAL_WORKER_BASE_URL?.trim() || undefined,
    temporalWorkerTimeoutMs: parsePositiveInt(process.env.BREVIO_TEMPORAL_WORKER_TIMEOUT_MS, 1500, 'BREVIO_TEMPORAL_WORKER_TIMEOUT_MS'),
    capabilityInventoryJson: process.env.BREVIO_CAPABILITY_INVENTORY_JSON?.trim() || undefined
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
    userId: getHeader(req, 'x-user-id')
  };
}

function logEvent(
  runtime: HandsRuntime,
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
      user_id: ctx.userId,
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

function asStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const normalized = value
    .map((item) => asString(item))
    .filter((item): item is string => Boolean(item));
  return normalized.length > 0 ? normalized : undefined;
}

function parseExecuteRequest(payload: Record<string, unknown>): ExecuteRequest {
  const skillId = asString(payload.skill_id);
  if (!skillId) {
    throw new Error('skill_id_required');
  }

  const input = asObject(payload.input) ?? {};
  const profile = asObject(payload.user_profile);
  const config = asObject(payload.config);
  const policyRaw = asObject(payload.policy);

  return {
    skill_id: skillId,
    ...parseExecutionRefs(payload),
    user_id: asString(payload.user_id),
    tenant_id: asString(payload.tenant_id),
    workspace_id: asString(payload.workspace_id),
    allowed_skills: asStringArray(payload.allowed_skills),
    input,
    user_profile: profile as Partial<UserProfile> | undefined,
    config,
    policy: policyRaw
      ? {
          consent_requirement: asString(policyRaw.consent_requirement) as ExecutionPolicy['consent_requirement'],
          consent_record: asString(policyRaw.consent_record),
          human_review: asString(policyRaw.human_review) as ExecutionPolicy['human_review'],
          human_review_record: asString(policyRaw.human_review_record),
          recipient_verification: asString(policyRaw.recipient_verification) as ExecutionPolicy['recipient_verification']
        }
      : undefined
  };
}

function getCircuit(runtime: HandsRuntime, skillId: string): CircuitStateEntry {
  const existing = runtime.circuits.get(skillId);
  if (existing) {
    return existing;
  }

  const created: CircuitStateEntry = {
    state: 'CLOSED',
    failureCount: 0,
    halfOpenRemaining: runtime.config.circuitHalfOpenMaxCalls,
    updatedAtMs: Date.now()
  };

  runtime.circuits.set(skillId, created);
  return created;
}

function checkCircuitBeforeExecution(runtime: HandsRuntime, skillId: string): CircuitStateEntry {
  const circuit = getCircuit(runtime, skillId);
  const now = Date.now();

  if (circuit.state === 'OPEN') {
    const openedAt = circuit.openedAtMs ?? 0;
    if (now - openedAt >= runtime.config.circuitRecoveryTimeoutMs) {
      circuit.state = 'HALF_OPEN';
      circuit.failureCount = 0;
      circuit.halfOpenRemaining = runtime.config.circuitHalfOpenMaxCalls;
      circuit.updatedAtMs = now;
    } else {
      return circuit;
    }
  }

  if (circuit.state === 'HALF_OPEN' && circuit.halfOpenRemaining > 0) {
    circuit.halfOpenRemaining -= 1;
    circuit.updatedAtMs = now;
  }

  return circuit;
}

function markExecutionSuccess(runtime: HandsRuntime, skillId: string): void {
  const circuit = getCircuit(runtime, skillId);
  circuit.state = 'CLOSED';
  circuit.failureCount = 0;
  circuit.halfOpenRemaining = runtime.config.circuitHalfOpenMaxCalls;
  circuit.openedAtMs = undefined;
  circuit.updatedAtMs = Date.now();
}

function markExecutionFailure(runtime: HandsRuntime, skillId: string): CircuitBreakerState {
  const circuit = getCircuit(runtime, skillId);
  const now = Date.now();

  circuit.failureCount += 1;

  if (circuit.state === 'HALF_OPEN' || circuit.failureCount >= runtime.config.circuitFailureThreshold) {
    circuit.state = 'OPEN';
    circuit.openedAtMs = now;
    circuit.halfOpenRemaining = runtime.config.circuitHalfOpenMaxCalls;
  } else {
    circuit.state = 'CLOSED';
  }

  circuit.updatedAtMs = now;
  return circuit.state;
}

function createSkillLogger(runtime: HandsRuntime, ctx: RequestContext, skillId: string): StructuredLogger {
  return {
    info(payload, message) {
      logEvent(runtime, ctx, 'hands.skill.info', 'INFO', {
        skill_id: skillId,
        message,
        payload
      });
    },
    warn(payload, message) {
      logEvent(runtime, ctx, 'hands.skill.warn', 'WARN', {
        skill_id: skillId,
        message,
        payload
      });
    },
    error(payload, message) {
      logEvent(runtime, ctx, 'hands.skill.error', 'ERROR', {
        skill_id: skillId,
        message,
        payload
      });
    }
  };
}

function createTracer(ctx: RequestContext): Tracer {
  return {
    startSpan(name: string): Record<string, unknown> {
      return {
        name,
        trace_id: ctx.traceId,
        span_id: ctx.spanId
      };
    }
  };
}

function createCacheClient(): CacheClient {
  return {
    async get(): Promise<string | null> {
      return null;
    },
    async set(): Promise<void> {
      return;
    }
  };
}

async function executeWithTimeout(
  adapter: { execute(input: SkillInput, ctx: SkillContext): Promise<SkillResult> },
  input: SkillInput,
  ctx: SkillContext,
  timeoutMs: number,
  skillId: string
): Promise<SkillResult> {
  let timer: NodeJS.Timeout | undefined;
  try {
    const timeoutPromise = new Promise<SkillResult>((_, reject) => {
      timer = setTimeout(() => {
        reject(new SkillExecutionTimeoutError(skillId, timeoutMs));
      }, timeoutMs);
    });

    const result = await Promise.race([adapter.execute(input, ctx), timeoutPromise]);
    return result;
  } finally {
    if (timer) {
      clearTimeout(timer);
    }
  }
}

function normalizeSkillResult(
  skillId: string,
  result: SkillResult,
  latencyMs: number,
  circuitState: CircuitBreakerState,
  refs: ExecutionRefs
): SkillResult {
  return applyExecutionRefs({
    skill_id: result.skill_id || skillId,
    status: result.status,
    data: result.data,
    error: result.error,
    latency_ms: Number.isInteger(result.latency_ms) ? result.latency_ms : latencyMs,
    tokens_used: result.tokens_used,
    cost_cents: result.cost_cents,
    metadata: {
      retries: result.metadata?.retries ?? 0,
      circuit_breaker_state: circuitState,
      cache_hit: result.metadata?.cache_hit ?? false
    }
  }, refs) as SkillResult;
}

function failureResult(
  skillId: string,
  status: SkillResult['status'],
  code: SkillResult['error'] extends infer E
    ? E extends { code: infer C }
      ? C
      : never
    : never,
  message: string,
  retryable: boolean,
  httpStatus: number,
  latencyMs: number,
  circuitState: CircuitBreakerState,
  refs: ExecutionRefs
): SkillResult {
  return applyExecutionRefs({
    skill_id: skillId,
    status,
    error: {
      code,
      message,
      retryable,
      http_status: httpStatus
    },
    latency_ms: latencyMs,
    metadata: {
      retries: 0,
      circuit_breaker_state: circuitState,
      cache_hit: false
    }
  }, refs) as SkillResult;
}

function healthPayload(runtime: HandsRuntime, deep: boolean): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    status: 'healthy',
    version: runtime.config.version,
    uptime_ms: Date.now() - runtime.startedAtMs
  };

  if (!deep) {
    return payload;
  }

  const circuitsOpen = Array.from(runtime.circuits.values()).filter((entry) => entry.state === 'OPEN').length;
  const circuitsHalfOpen = Array.from(runtime.circuits.values()).filter((entry) => entry.state === 'HALF_OPEN').length;

  return {
    ...payload,
    checks: {
      process: 'ok',
      db: process.env.DATABASE_URL ? 'configured' : 'not_configured',
      redis: process.env.REDIS_URL ? 'configured' : 'not_configured',
      temporal: process.env.TEMPORAL_HOST ? 'configured' : 'not_configured',
      skill_registry: runtime.skills.length > 0 ? 'loaded' : 'empty'
    },
    skills: {
      total: runtime.skills.length
    },
    circuit_breakers: {
      open: circuitsOpen,
      half_open: circuitsHalfOpen,
      tracked: runtime.circuits.size
    }
  };
}

function listSkills(runtime: HandsRuntime): Record<string, unknown> {
  return {
    total: runtime.skills.length,
    skills: runtime.skills
  };
}

function circuitSnapshot(runtime: HandsRuntime): Record<string, unknown> {
  const entries = Array.from(runtime.circuits.entries()).map(([skillId, state]) => ({
    skill_id: skillId,
    state: state.state,
    failure_count: state.failureCount,
    opened_at_ms: state.openedAtMs,
    half_open_remaining: state.halfOpenRemaining,
    updated_at_ms: state.updatedAtMs
  }));

  return {
    total: entries.length,
    entries
  };
}

function agentCardPayload(runtime: HandsRuntime): Record<string, unknown> {
  return {
    agent_id: runtime.config.serviceName,
    name: 'Brevio Hands',
    description: 'Execution-plane service for approved Brevio skills with circuit breakers, approval enforcement, and workflow result reporting.',
    version: runtime.config.version,
    protocol_version: '2026.a2a.v1',
    default_endpoint: `http://localhost:${runtime.config.port}/api/v1/hands`,
    capabilities: runtime.skills.map((skillId) => ({
      id: skillId,
      name: skillId,
      description: `Hands execution capability for ${skillId}.`,
      version: '1.0.0',
      input_modes: ['application/json'],
      output_modes: ['application/json'],
      async: true
    })),
    supports: {
      task_lifecycle: false,
      task_query: false,
      artifact_updates: true,
      push_callbacks: false,
      capability_inventory: true
    }
  };
}

function buildRuntime(config?: HandsConfig): HandsRuntime {
  const resolvedConfig = config ?? loadConfig();
  const skills = Object.entries(SkillRegistry)
    .filter(([, adapter]) => isHandsExecutableAdapter(adapter))
    .map(([skillId]) => skillId)
    .sort();
  const circuits = new Map<string, CircuitStateEntry>();
  const startedAtMs = Date.now();

  let runtimeRef: HandsRuntime | undefined;

  const server = http.createServer((req, res) => {
    const runtime = runtimeRef;
    if (!runtime) {
      sendJSON(res, 500, { error: 'runtime_not_ready' });
      return;
    }

    const method = req.method ?? 'GET';
    const path = new URL(req.url ?? '/', 'http://localhost').pathname;
    const ctx = requestContext(req);

    const executePaths = new Set([
      '/v1/hands/execute',
      '/api/v1/hands/execute',
      '/v1/hands/tool/execute',
      '/api/v1/hands/tool/execute'
    ]);

    const skillsPaths = new Set(['/v1/hands/skills', '/api/v1/hands/skills']);

    const onError = (statusCode: number, code: string): void => {
      sendJSON(res, statusCode, { error: code });
      logEvent(runtime, ctx, 'hands.request.error', 'WARN', {
        method,
        path,
        status_code: statusCode,
        code
      });
    };

    if (method === 'GET' && path === '/health') {
      sendJSON(res, 200, healthPayload(runtime, false));
      return;
    }

    if (method === 'GET' && path === '/health/deep') {
      sendJSON(res, 200, healthPayload(runtime, true));
      return;
    }

    if (method === 'GET' && path === '/.well-known/agent-card.json') {
      sendJSON(res, 200, agentCardPayload(runtime));
      return;
    }

    if (method === 'GET' && skillsPaths.has(path)) {
      sendJSON(res, 200, listSkills(runtime));
      return;
    }

    if (method === 'GET' && path === '/api/v1/hands/circuit-breakers') {
      sendJSON(res, 200, circuitSnapshot(runtime));
      return;
    }

    if (method === 'POST' && executePaths.has(path)) {
      void (async () => {
        const started = Date.now();
        const rawBody = await readRawBody(req, runtime.config.maxBodyBytes);
        const payload = parseObject(rawBody);
        const parsed = parseExecuteRequest(payload);

        const adapter = getSkillAdapter(parsed.skill_id);
        if (!adapter) {
          const result = failureResult(
            parsed.skill_id,
            'FAILED',
            'SKILL_NOT_FOUND',
            `skill ${parsed.skill_id} is not registered`,
            false,
            404,
            Date.now() - started,
            'CLOSED',
            parsed
          );
          sendJSON(res, 404, result as unknown as Record<string, unknown>);
          logEvent(runtime, ctx, 'hands.execute.not_found', 'WARN', {
            skill_id: parsed.skill_id
          });
          return;
        }

        const approvalGate = evaluateApprovalGate(parsed.policy);
        if (approvalGate) {
          const blocked = failureResult(
            parsed.skill_id,
            'FAILED',
            approvalGate.code,
            approvalGate.message,
            false,
            approvalGate.httpStatus,
            Date.now() - started,
            'CLOSED',
            parsed
          );
          const workflowReport = await reportExecutionResult(blocked, runtime.config);
          sendJSON(res, approvalGate.httpStatus, blocked as unknown as Record<string, unknown>);
          logEvent(runtime, ctx, 'hands.execute.approval_required', 'WARN', {
            skill_id: parsed.skill_id,
            approval_code: approvalGate.code,
            workflow_runtime_delegated: workflowReport.delegated,
            workflow_runtime_warning: workflowReport.warning
          });
          return;
        }

        const circuit = checkCircuitBeforeExecution(runtime, parsed.skill_id);
        if (circuit.state === 'OPEN') {
          const result = failureResult(
            parsed.skill_id,
            'FAILED',
            'CIRCUIT_OPEN',
            `skill ${parsed.skill_id} circuit breaker is open`,
            true,
            503,
            Date.now() - started,
            'OPEN',
            parsed
          );
          sendJSON(res, 503, result as unknown as Record<string, unknown>);
          logEvent(runtime, ctx, 'hands.execute.circuit_open', 'WARN', {
            skill_id: parsed.skill_id,
            failure_count: circuit.failureCount
          });
          return;
        }

        const userId = parsed.user_id ?? ctx.userId ?? randomUUID();
        const userProfile: UserProfile = {
          id: userId,
          timezone: parsed.user_profile?.timezone ?? 'UTC',
          locale: parsed.user_profile?.locale ?? 'en-US'
        };
        const requestResolvedSkills = parsed.allowed_skills ?? parsed.user_profile?.enabled_skills;
        const capabilityResolution = requestResolvedSkills && requestResolvedSkills.length > 0
          ? {
              enabledSkills: requestResolvedSkills,
              deniedSkills: [],
              source: 'explicit' as const
            }
          : resolveCapabilityInventory(
              parseCapabilityInventory(runtime.config.capabilityInventoryJson),
              {
                tenantId: parsed.tenant_id,
                workspaceId: parsed.workspace_id,
                userId
              }
            );

        if (capabilityResolution.source !== 'none' && !capabilityResolution.enabledSkills.includes(parsed.skill_id)) {
          const blocked = failureResult(
            parsed.skill_id,
            'FAILED',
            'CAPABILITY_NOT_ENABLED',
            `skill ${parsed.skill_id} is not enabled for this user capability inventory`,
            false,
            403,
            Date.now() - started,
            'CLOSED',
            parsed
          );
          const workflowReport = await reportExecutionResult(blocked, runtime.config);
          sendJSON(res, 403, blocked as unknown as Record<string, unknown>);
          logEvent(runtime, ctx, 'hands.execute.capability_blocked', 'WARN', {
            skill_id: parsed.skill_id,
            capability_source: capabilityResolution.source,
            denied_skills: capabilityResolution.deniedSkills,
            workflow_runtime_delegated: workflowReport.delegated,
            workflow_runtime_warning: workflowReport.warning
          });
          return;
        }

        const skillContext: SkillContext = {
          userId,
          oauthTokens: new Map(),
          userProfile,
          logger: createSkillLogger(runtime, ctx, parsed.skill_id),
          tracer: createTracer(ctx),
          cache: createCacheClient(),
          config: parsed.config ?? {}
        };

        try {
          const rawResult = await executeWithTimeout(
            adapter,
            parsed.input,
            skillContext,
            runtime.config.executionTimeoutMs,
            parsed.skill_id
          );

          markExecutionSuccess(runtime, parsed.skill_id);

          const normalized = normalizeSkillResult(
            parsed.skill_id,
            rawResult,
            Date.now() - started,
            getCircuit(runtime, parsed.skill_id).state,
            parsed
          );

          const workflowReport = await reportExecutionResult(normalized, runtime.config);

          sendJSON(res, 200, normalized as unknown as Record<string, unknown>);
          logEvent(runtime, ctx, 'hands.execute.complete', 'INFO', {
            request_id: parsed.request_id,
            run_id: parsed.run_id,
            task_id: parsed.task_id,
            step_id: parsed.step_id,
            attempt: parsed.attempt,
            skill_id: normalized.skill_id,
            status: normalized.status,
            latency_ms: normalized.latency_ms,
            workflow_runtime_delegated: workflowReport.delegated,
            workflow_runtime_warning: workflowReport.warning
          });
          return;
        } catch (err) {
          const stateAfterFailure = markExecutionFailure(runtime, parsed.skill_id);

          if (err instanceof SkillExecutionTimeoutError) {
            const timeoutResult = failureResult(
              parsed.skill_id,
              'TIMEOUT',
              'EXTERNAL_TIMEOUT',
              err.message,
              true,
              504,
              Date.now() - started,
              stateAfterFailure,
              parsed
            );
            const workflowReport = await reportExecutionResult(timeoutResult, runtime.config);
            sendJSON(res, 504, timeoutResult as unknown as Record<string, unknown>);
            logEvent(runtime, ctx, 'hands.execute.timeout', 'WARN', {
              skill_id: parsed.skill_id,
              circuit_breaker_state: stateAfterFailure,
              timeout_ms: runtime.config.executionTimeoutMs,
              workflow_runtime_delegated: workflowReport.delegated,
              workflow_runtime_warning: workflowReport.warning
            });
            return;
          }

          const failure = failureResult(
            parsed.skill_id,
            'FAILED',
            'EXTERNAL_ERROR',
            err instanceof Error ? err.message : 'skill execution failed',
            true,
            502,
            Date.now() - started,
            stateAfterFailure,
            parsed
          );
          const workflowReport = await reportExecutionResult(failure, runtime.config);
          sendJSON(res, 502, failure as unknown as Record<string, unknown>);
          logEvent(runtime, ctx, 'hands.execute.failed', 'ERROR', {
            skill_id: parsed.skill_id,
            circuit_breaker_state: stateAfterFailure,
            message: err instanceof Error ? err.message : String(err),
            workflow_runtime_delegated: workflowReport.delegated,
            workflow_runtime_warning: workflowReport.warning
          });
          return;
        }
      })().catch((err) => {
        if (err instanceof Error && err.message === 'payload_too_large') {
          onError(413, 'payload_too_large');
          return;
        }
        if (err instanceof Error && err.message === 'invalid_json') {
          onError(400, 'invalid_json');
          return;
        }
        if (err instanceof Error && err.message === 'skill_id_required') {
          onError(400, 'skill_id_required');
          return;
        }
        onError(500, 'execute_failed');
        logEvent(runtime, ctx, 'hands.execute.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    onError(404, 'not_found');
  });

  const runtime: HandsRuntime = {
    config: resolvedConfig,
    startedAtMs,
    server,
    skills,
    circuits,
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

function installSignalHandlers(runtime: HandsRuntime): void {
  const shutdown = async (signal: string): Promise<void> => {
    const ctx: RequestContext = {
      traceId: randomUUID(),
      spanId: randomUUID(),
      requestId: randomUUID()
    };

    logEvent(runtime, ctx, 'hands.shutdown.start', 'INFO', { signal });

    const timeout = setTimeout(() => {
      logEvent(runtime, ctx, 'hands.shutdown.timeout', 'ERROR', {
        timeout_ms: runtime.config.shutdownTimeoutMs
      });
      process.exit(1);
    }, runtime.config.shutdownTimeoutMs);

    try {
      await runtime.close();
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'hands.shutdown.complete', 'INFO', {});
      process.exit(0);
    } catch (err) {
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'hands.shutdown.failed', 'ERROR', {
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

  logEvent(runtime, ctx, 'hands.started', 'INFO', {
    port: runtime.config.port,
    skills: runtime.skills.length,
    execution_timeout_ms: runtime.config.executionTimeoutMs,
    circuit_failure_threshold: runtime.config.circuitFailureThreshold,
    circuit_recovery_timeout_ms: runtime.config.circuitRecoveryTimeoutMs
  });
}

if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url) {
  void main().catch((err) => {
    process.stderr.write(
      JSON.stringify({
        ts: new Date().toISOString(),
        service: 'brevio-hands',
        event: 'hands.start.failed',
        severity: 'ERROR',
        message: err instanceof Error ? err.message : String(err)
      }) + '\n'
    );
    process.exit(1);
  });
}

export { buildRuntime as createHandsRuntime };
