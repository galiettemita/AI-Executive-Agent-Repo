import { randomUUID } from 'node:crypto';
import http from 'node:http';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

import { parseCapabilityInventory, resolveCapabilityInventory } from '../../../packages/shared/src/capability-inventory.js';
import {
  extractBearerToken,
  loadBrevioEnvironment,
  pseudonymizedRef,
  requireSharedSecret,
  verifyAccessToken,
  verifyCallerContextEnvelope
} from '../../../packages/shared/src/security.js';
import type {
  CacheClient,
  SkillContext,
  SkillInput,
  SkillResult,
  StructuredLogger,
  Tracer,
  UserProfile
} from '@brevio/shared';

import { deriveRequiredPolicy, evaluateApprovalGate, mergeExecutionPolicy, type ExecutionPolicy } from './approval-policy.js';
import { buildToolKey, getToolDescriptor, isRegisteredOperation } from '../../brevio-brain/src/catalog.js';
import { CircuitStore, type CircuitBreakerState, type CircuitStateEntry } from './circuit-store.js';
import { applyExecutionRefs, parseExecutionRefs, type ExecutionRefs } from './execution-refs.js';
import { isHandsExecutableAdapter } from './skills/plane-policy.js';
import { getSkillAdapter, SkillRegistry } from './skills/index.js';
import { reportExecutionResult } from './workflow-runtime.js';

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
  stateFilePath?: string;
  temporalWorkerBaseUrl?: string;
  temporalWorkerTimeoutMs: number;
  capabilityInventoryJson?: string;
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

interface ExecuteRequest {
  skill_id: string;
  tool?: string;
  operation?: string;
  request_id?: string;
  run_id?: string;
  task_id?: string;
  step_id?: string;
  attempt?: number;
  user_id?: string;
  tenant_id?: string;
  workspace_id?: string;
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
  circuitStore: CircuitStore;
  close(): Promise<void>;
}

interface AuthenticatedIdentity {
  subject: string;
  userId?: string;
  admin: boolean;
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
  const environment = loadBrevioEnvironment();
  return {
    serviceName: 'brevio-hands',
    version: process.env.SERVICE_VERSION ?? '0.2.0',
    environment,
    port: parsePositiveInt(process.env.PORT, 8082, 'PORT'),
    shutdownTimeoutMs: parsePositiveInt(process.env.BREVIO_HANDS_SHUTDOWN_TIMEOUT_MS, 30000, 'BREVIO_HANDS_SHUTDOWN_TIMEOUT_MS'),
    executionTimeoutMs: parsePositiveInt(process.env.BREVIO_HANDS_EXECUTION_TIMEOUT_MS, 60000, 'BREVIO_HANDS_EXECUTION_TIMEOUT_MS'),
    maxBodyBytes: parsePositiveInt(process.env.BREVIO_HANDS_MAX_BODY_BYTES, 2 * 1024 * 1024, 'BREVIO_HANDS_MAX_BODY_BYTES'),
    circuitFailureThreshold: parsePositiveInt(process.env.BREVIO_HANDS_CB_FAILURE_THRESHOLD, 5, 'BREVIO_HANDS_CB_FAILURE_THRESHOLD'),
    circuitRecoveryTimeoutMs: parsePositiveInt(process.env.BREVIO_HANDS_CB_RECOVERY_TIMEOUT_MS, 60000, 'BREVIO_HANDS_CB_RECOVERY_TIMEOUT_MS'),
    circuitHalfOpenMaxCalls: parsePositiveInt(process.env.BREVIO_HANDS_CB_HALF_OPEN_MAX_CALLS, 3, 'BREVIO_HANDS_CB_HALF_OPEN_MAX_CALLS'),
    stateFilePath: path.resolve(process.env.BREVIO_HANDS_STATE_FILE ?? path.join(process.cwd(), 'data', 'hands', 'circuit-state.json')),
    temporalWorkerBaseUrl: process.env.BREVIO_TEMPORAL_WORKER_BASE_URL?.trim() || undefined,
    temporalWorkerTimeoutMs: parsePositiveInt(process.env.BREVIO_TEMPORAL_WORKER_TIMEOUT_MS, 1500, 'BREVIO_TEMPORAL_WORKER_TIMEOUT_MS'),
    capabilityInventoryJson: process.env.BREVIO_CAPABILITY_INVENTORY_JSON?.trim() || undefined,
    internalAuthSecret: requireSharedSecret(process.env.BREVIO_INTERNAL_AUTH_SECRET, 'BREVIO_INTERNAL_AUTH_SECRET', environment, 'brevio-hands'),
    internalAuthIssuer: process.env.BREVIO_INTERNAL_AUTH_ISSUER?.trim() || 'https://auth.brevio.internal',
    serviceAudience: process.env.BREVIO_HANDS_AUDIENCE?.trim() || 'brevio-hands',
    callerContextSecret: requireSharedSecret(process.env.BREVIO_CALLER_CONTEXT_SECRET, 'BREVIO_CALLER_CONTEXT_SECRET', environment, 'brevio-hands-caller'),
    logSalt: process.env.BREVIO_HANDS_LOG_SALT?.trim() || `brevio-hands:${environment}`
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
      subject_ref: ctx.subjectRef,
      event,
      severity,
      attrs
    }) + '\n'
  );
}

function authenticateRequest(
  req: http.IncomingMessage,
  runtime: HandsRuntime,
  ctx: RequestContext,
  mode: 'api' | 'admin'
): AuthenticatedIdentity {
  const token = extractBearerToken(getHeader(req, 'authorization'));
  if (!token) {
    throw new Error('authorization_required');
  }
  const principal = verifyAccessToken(runtime.config.internalAuthSecret, token, {
    expectedAudience: runtime.config.serviceAudience,
    expectedIssuer: runtime.config.internalAuthIssuer,
    allowedTokenUses: mode === 'admin' ? ['admin_access'] : ['service_access', 'admin_access', 'user_access']
  });
  ctx.subjectRef = pseudonymizedRef(principal.sub, runtime.config.logSalt);
  return {
    subject: principal.sub,
    userId: principal.token_use === 'user_access' || principal.token_use === 'admin_access' ? principal.sub : undefined,
    admin: principal.token_use === 'admin_access'
  };
}

function callerContextFromRequest(req: http.IncomingMessage, runtime: HandsRuntime): ReturnType<typeof verifyCallerContextEnvelope> | null {
  const header = getHeader(req, 'x-brevio-caller-context');
  if (!header) {
    return null;
  }
  return verifyCallerContextEnvelope(runtime.config.callerContextSecret, header);
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
    tool: asString(payload.tool),
    operation: asString(payload.operation),
    ...parseExecutionRefs(payload),
    user_id: asString(payload.user_id),
    tenant_id: asString(payload.tenant_id),
    workspace_id: asString(payload.workspace_id),
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

function resolveExecutionContract(parsed: ExecuteRequest): { tool: string; operation: string } {
  const descriptor = getToolDescriptor(parsed.skill_id);
  if (!descriptor) {
    throw new Error('skill_contract_missing');
  }
  const requestedOperation =
    parsed.operation ??
    (typeof parsed.input.action === 'string' && parsed.input.action.trim() !== '' ? parsed.input.action.trim() : undefined) ??
    (descriptor.operations.length === 1 ? descriptor.operations[0] : undefined);
  if (!requestedOperation || !isRegisteredOperation(parsed.skill_id, requestedOperation)) {
    throw new Error('invalid_operation_contract');
  }
  const expectedTool = buildToolKey(parsed.skill_id, requestedOperation);
  if (parsed.tool && parsed.tool !== expectedTool) {
    throw new Error('tool_operation_mismatch');
  }
  return {
    tool: expectedTool,
    operation: requestedOperation
  };
}

function getCircuit(runtime: HandsRuntime, skillId: string): CircuitStateEntry {
  return runtime.circuitStore.get(skillId, runtime.config.circuitHalfOpenMaxCalls);
}

function checkCircuitBeforeExecution(runtime: HandsRuntime, skillId: string): CircuitStateEntry {
  const now = Date.now();
  return runtime.circuitStore.update(
    skillId,
    runtime.config.circuitHalfOpenMaxCalls,
    (circuit) => {
      if (circuit.state === 'OPEN') {
        const openedAt = circuit.openedAtMs ?? 0;
        if (now - openedAt >= runtime.config.circuitRecoveryTimeoutMs) {
          circuit.state = 'HALF_OPEN';
          circuit.failureCount = 0;
          circuit.halfOpenRemaining = runtime.config.circuitHalfOpenMaxCalls;
          circuit.updatedAtMs = now;
        }
      }

      if (circuit.state === 'HALF_OPEN' && circuit.halfOpenRemaining > 0) {
        circuit.halfOpenRemaining -= 1;
        circuit.updatedAtMs = now;
      }
    },
    now
  );
}

function markExecutionSuccess(runtime: HandsRuntime, skillId: string): void {
  runtime.circuitStore.update(skillId, runtime.config.circuitHalfOpenMaxCalls, (circuit) => {
    circuit.state = 'CLOSED';
    circuit.failureCount = 0;
    circuit.halfOpenRemaining = runtime.config.circuitHalfOpenMaxCalls;
    circuit.openedAtMs = undefined;
    circuit.updatedAtMs = Date.now();
  });
}

function markExecutionFailure(runtime: HandsRuntime, skillId: string): CircuitBreakerState {
  const now = Date.now();
  return runtime.circuitStore.update(
    skillId,
    runtime.config.circuitHalfOpenMaxCalls,
    (circuit) => {
      circuit.failureCount += 1;

      if (circuit.state === 'HALF_OPEN' || circuit.failureCount >= runtime.config.circuitFailureThreshold) {
        circuit.state = 'OPEN';
        circuit.openedAtMs = now;
        circuit.halfOpenRemaining = runtime.config.circuitHalfOpenMaxCalls;
      } else {
        circuit.state = 'CLOSED';
      }

      circuit.updatedAtMs = now;
    },
    now
  ).state;
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
    execution_receipt:
      result.execution_receipt ??
      (result.status === 'SUCCESS'
        ? {
            executor: 'brevio-hands',
            mode: 'delegated',
            issued_at: new Date().toISOString(),
            receipt_id: `hands:${refs.request_id ?? skillId}:${Date.now()}`
          }
        : undefined),
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

  const circuitEntries = runtime.circuitStore.entries().map(([, entry]) => entry);
  const circuitsOpen = circuitEntries.filter((entry) => entry.state === 'OPEN').length;
  const circuitsHalfOpen = circuitEntries.filter((entry) => entry.state === 'HALF_OPEN').length;

  return {
    ...payload,
    checks: {
      process: 'ok',
      db: process.env.DATABASE_URL ? 'configured' : 'not_configured',
      redis: process.env.REDIS_URL ? 'configured' : 'not_configured',
      temporal: process.env.TEMPORAL_HOST ? 'configured' : 'not_configured',
      skill_registry: runtime.skills.length > 0 ? 'loaded' : 'empty',
      circuit_store_mode: runtime.circuitStore.mode(),
      circuit_store_path: runtime.circuitStore.snapshotPath()
    },
    skills: {
      total: runtime.skills.length
    },
    circuit_breakers: {
      open: circuitsOpen,
      half_open: circuitsHalfOpen,
      tracked: runtime.circuitStore.size()
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
  const entries = runtime.circuitStore.entries().map(([skillId, state]) => ({
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
  const mediaModesForSkill = (skillId: string): { input: string[]; output: string[] } => {
    if (['asr', 'gemini-stt', 'vocal-chat'].includes(skillId)) {
      return { input: ['application/json', 'audio/*'], output: ['application/json', 'text/plain'] };
    }
    if (['openai-tts', 'voice-wake-say', 'sag'].includes(skillId)) {
      return { input: ['application/json', 'text/plain'], output: ['application/json', 'audio/*'] };
    }
    if (['pdf-tools'].includes(skillId)) {
      return { input: ['application/json', 'application/pdf'], output: ['application/json', 'text/plain', 'application/pdf'] };
    }
    if (['video-frames', 'video-transcript-downloader', 'veo'].includes(skillId)) {
      return { input: ['application/json', 'video/*'], output: ['application/json', 'image/*', 'video/*', 'text/plain'] };
    }
    if (['fal-ai', 'krea-api', 'pollinations', 'coloring-page', 'camsnap', 'apple-photos', 'apple-media'].includes(skillId)) {
      return { input: ['application/json', 'image/*', 'text/plain'], output: ['application/json', 'image/*'] };
    }
    return { input: ['application/json'], output: ['application/json'] };
  };

  return {
    agent_id: runtime.config.serviceName,
    name: 'Brevio Hands',
    description: 'Execution-plane service for approved Brevio skills with circuit breakers, approval enforcement, and workflow result reporting.',
    version: runtime.config.version,
    protocol_version: '2026.a2a.v1',
    default_endpoint: `http://localhost:${runtime.config.port}/api/v1/hands`,
    capabilities: runtime.skills.map((skillId) => {
      const modes = mediaModesForSkill(skillId);
      return {
        id: skillId,
        name: skillId,
        description: `Hands execution capability for ${skillId}.`,
        version: '1.0.0',
        input_modes: modes.input,
        output_modes: modes.output,
        async: true
      };
    }),
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
  const circuitStore = new CircuitStore(resolvedConfig.stateFilePath);
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
      try {
        authenticateRequest(req, runtime, ctx, 'admin');
      } catch (error) {
        onError(401, error instanceof Error ? error.message : 'authorization_required');
        return;
      }
      sendJSON(res, 200, healthPayload(runtime, true));
      return;
    }

    if (method === 'GET' && path === '/.well-known/agent-card.json') {
      sendJSON(res, 200, agentCardPayload(runtime));
      return;
    }

    if (method === 'GET' && skillsPaths.has(path)) {
      try {
        authenticateRequest(req, runtime, ctx, 'admin');
      } catch (error) {
        onError(401, error instanceof Error ? error.message : 'authorization_required');
        return;
      }
      sendJSON(res, 200, listSkills(runtime));
      return;
    }

    if (method === 'GET' && path === '/api/v1/hands/circuit-breakers') {
      try {
        authenticateRequest(req, runtime, ctx, 'admin');
      } catch (error) {
        onError(401, error instanceof Error ? error.message : 'authorization_required');
        return;
      }
      sendJSON(res, 200, circuitSnapshot(runtime));
      return;
    }

    if (method === 'POST' && executePaths.has(path)) {
      void (async () => {
        let identity: AuthenticatedIdentity;
        try {
          identity = authenticateRequest(req, runtime, ctx, 'api');
        } catch (error) {
          onError(401, error instanceof Error ? error.message : 'authorization_required');
          return;
        }
        const started = Date.now();
        const rawBody = await readRawBody(req, runtime.config.maxBodyBytes);
        const payload = parseObject(rawBody);
        const parsed = parseExecuteRequest(payload);
        let callerContext: ReturnType<typeof callerContextFromRequest>;
        try {
          callerContext = callerContextFromRequest(req, runtime);
        } catch (error) {
          onError(401, error instanceof Error ? error.message : 'invalid_caller_context');
          return;
        }

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

        let contract: { tool: string; operation: string };
        try {
          contract = resolveExecutionContract(parsed);
        } catch (error) {
          const validation = failureResult(
            parsed.skill_id,
            'FAILED',
            'VALIDATION_FAILED',
            error instanceof Error ? error.message : 'invalid execution contract',
            false,
            400,
            Date.now() - started,
            'CLOSED',
            parsed
          );
          const workflowReport = await reportExecutionResult(validation, runtime.config);
          sendJSON(res, 400, validation as unknown as Record<string, unknown>);
          logEvent(runtime, ctx, 'hands.execute.invalid_contract', 'WARN', {
            skill_id: parsed.skill_id,
            workflow_runtime_delegated: workflowReport.delegated,
            workflow_runtime_warning: workflowReport.warning
          });
          return;
        }

        const requiredPolicy = deriveRequiredPolicy(getToolDescriptor(parsed.skill_id), contract.operation);
        const effectivePolicy = mergeExecutionPolicy(requiredPolicy, parsed.policy);
        const approvalGate = evaluateApprovalGate(effectivePolicy);
        if (approvalGate) {
          const blocked = failureResult(
            parsed.skill_id,
            'NEEDS_CONSENT',
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

        const userId = callerContext?.user_id ?? identity.userId;
        if (!userId) {
          onError(400, 'caller_context_required');
          return;
        }
        const userProfile: UserProfile = {
          id: userId,
          timezone: parsed.user_profile?.timezone ?? 'UTC',
          locale: parsed.user_profile?.locale ?? 'en-US'
        };
        const capabilityResolution = resolveCapabilityInventory(
          parseCapabilityInventory(runtime.config.capabilityInventoryJson),
          {
            tenantId: callerContext?.tenant_id,
            workspaceId: callerContext?.workspace_id,
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
            tool: contract.tool,
            operation: contract.operation,
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
          const executorInput: SkillInput = {
            ...parsed.input,
            action: contract.operation
          };
          const rawResult = await executeWithTimeout(
            adapter,
            executorInput,
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
            tool: contract.tool,
            operation: contract.operation,
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
    circuitStore,
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
