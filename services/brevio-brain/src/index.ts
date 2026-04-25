import { randomUUID } from 'node:crypto';
import http from 'node:http';
import { pathToFileURL } from 'node:url';

import {
  extractBearerToken,
  pseudonymizedRef,
  verifyAccessToken,
  verifyCallerContextEnvelope
} from '../../../packages/shared/src/security.js';
import { aggregateResults } from './aggregate.js';
import { classifyIntent } from './classify.js';
import { loadBrainConfig, loadDisambiguationRules } from './config.js';
import { decomposeTask } from './decompose.js';
import { disambiguateSkills } from './disambiguate.js';
import { normalizeReasoningInput } from './normalize.js';
import { buildPlannerProposal } from './planner.js';
import type {
  AggregationRequest,
  BrainConfig,
  DisambiguationRequest,
  DisambiguationRules,
  IntentClassificationInput,
  NormalizedReasoningRequest,
  ProcessRequest,
  RequestContext,
  SkillResult
} from './types.js';
import { verifyPlan } from './verify.js';
import { annotateRunVerification, registerExecutionPlan, syncProcessRunState } from './workflow-runtime.js';

interface BrainRuntime {
  config: BrainConfig;
  startedAtMs: number;
  disambiguationRules: DisambiguationRules;
  server: http.Server;
  close(): Promise<void>;
}

interface AuthenticatedIdentity {
  subject: string;
  userId?: string;
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
  runtime: BrainRuntime,
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

function authenticateRequest(runtime: BrainRuntime, req: http.IncomingMessage, ctx: RequestContext, mode: 'api' | 'admin'): AuthenticatedIdentity {
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
    userId: principal.token_use === 'user_access' || principal.token_use === 'admin_access' ? principal.sub : undefined
  };
}

function callerContextFromRequest(runtime: BrainRuntime, req: http.IncomingMessage) {
  const token = getHeader(req, 'x-brevio-caller-context');
  return token ? verifyCallerContextEnvelope(runtime.config.callerContextSecret, token) : undefined;
}

function sendJSON(res: http.ServerResponse, statusCode: number, payload: Record<string, unknown>): void {
  res.writeHead(statusCode, { 'content-type': 'application/json' });
  res.end(JSON.stringify(payload));
}

async function readRawBody(req: http.IncomingMessage, maxBytes = 2 * 1024 * 1024): Promise<Buffer> {
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

function parseObject(rawBody: Buffer): { value?: Record<string, unknown>; error?: string } {
  try {
    const parsed = JSON.parse(rawBody.toString('utf8'));
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return { value: parsed as Record<string, unknown> };
    }
    return { error: 'invalid_json_object' };
  } catch {
    return { error: 'invalid_json' };
  }
}

function asString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function asStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const output = value
    .map((item) => asString(item))
    .filter((item): item is string => Boolean(item));
  return output.length > 0 ? [...new Set(output)] : undefined;
}

function asBool(value: unknown): boolean | undefined {
  if (typeof value === 'boolean') {
    return value;
  }
  return undefined;
}

function asExecutionReceiptMode(
  value: unknown
): SkillResult['execution_receipt'] extends infer Receipt
  ? Receipt extends { mode: infer Mode }
    ? Mode
    : never
  : never {
  return value === 'direct' || value === 'delegated' || value === 'local' || value === 'simulated'
    ? value
    : 'delegated';
}

function extractSkillResults(value: unknown): SkillResult[] {
  if (!Array.isArray(value)) {
    return [];
  }

  const out: SkillResult[] = [];
  for (const item of value) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) {
      continue;
    }
    const raw = item as Record<string, unknown>;
    const skillId = asString(raw.skill_id);
    const status = asString(raw.status);
    if (!skillId || !status) {
      continue;
    }
    if (
      status !== 'SUCCESS' &&
      status !== 'PARTIAL' &&
      status !== 'FAILED' &&
      status !== 'TIMEOUT' &&
      status !== 'NEEDS_CONSENT' &&
      status !== 'NOT_EXECUTED' &&
      status !== 'SIMULATED'
    ) {
      continue;
    }

    out.push({
      request_id: asString(raw.request_id),
      run_id: asString(raw.run_id),
      task_id: asString(raw.task_id),
      step_id: asString(raw.step_id),
      attempt: typeof raw.attempt === 'number' && Number.isInteger(raw.attempt) && raw.attempt > 0 ? raw.attempt : undefined,
      skill_id: skillId,
      status,
      data: raw.data && typeof raw.data === 'object' && !Array.isArray(raw.data) ? (raw.data as Record<string, unknown>) : undefined,
      error: raw.error && typeof raw.error === 'object' && !Array.isArray(raw.error)
        ? {
            code: asString((raw.error as Record<string, unknown>).code) ?? 'UNKNOWN_ERROR',
            message: asString((raw.error as Record<string, unknown>).message) ?? 'unknown error'
          }
        : undefined,
      execution_receipt: raw.execution_receipt && typeof raw.execution_receipt === 'object' && !Array.isArray(raw.execution_receipt)
        ? {
            executor: asString((raw.execution_receipt as Record<string, unknown>).executor) ?? 'unknown',
            mode: asExecutionReceiptMode((raw.execution_receipt as Record<string, unknown>).mode),
            issued_at: asString((raw.execution_receipt as Record<string, unknown>).issued_at) ?? new Date().toISOString(),
            receipt_id: asString((raw.execution_receipt as Record<string, unknown>).receipt_id) ?? `missing-${skillId}`
          }
        : undefined,
      source: asString(raw.source) === 'external' ? 'external' : 'hands'
    });
  }

  return out;
}

function normalizePayload(
  payload: Record<string, unknown>,
  identity?: AuthenticatedIdentity,
  callerContext?: ReturnType<typeof callerContextFromRequest>
): NormalizedReasoningRequest {
  const request: ProcessRequest = {
    message_text: asString(payload.message_text) ?? '',
    run_id: asString(payload.run_id),
    thread_id: asString(payload.thread_id),
    workspace_id: callerContext?.workspace_id,
    user_id: callerContext?.user_id ?? identity?.userId,
    user_profile:
      payload.user_profile && typeof payload.user_profile === 'object' && !Array.isArray(payload.user_profile)
        ? (payload.user_profile as ProcessRequest['user_profile'])
        : undefined,
    user_preferences:
      payload.user_preferences && typeof payload.user_preferences === 'object' && !Array.isArray(payload.user_preferences)
        ? (payload.user_preferences as ProcessRequest['user_preferences'])
        : undefined,
    deployment_mode: asString(payload.deployment_mode) as ProcessRequest['deployment_mode'],
    user_tier: asString(payload.user_tier) as ProcessRequest['user_tier'],
    channel: asString(payload.channel) as ProcessRequest['channel'],
    context:
      payload.context && typeof payload.context === 'object' && !Array.isArray(payload.context)
        ? (payload.context as ProcessRequest['context'])
        : undefined,
    skill_results: extractSkillResults(payload.skill_results)
  };
  return normalizeReasoningInput(request);
}

function healthPayload(runtime: BrainRuntime, deep: boolean): Record<string, unknown> {
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
      disambiguation_config: runtime.config.disambiguationConfigPath,
      planner_provider: runtime.config.plannerProvider,
      planner_model: runtime.config.plannerModel
    },
    disambiguation: {
      rules_loaded: Object.keys(runtime.disambiguationRules).length,
      groups: Object.keys(runtime.disambiguationRules).sort()
    }
  };
}

function agentCardPayload(runtime: BrainRuntime): Record<string, unknown> {
  return {
    agent_id: runtime.config.serviceName,
    name: 'Brevio Brain',
    description: 'Planning and reasoning service for Brevio A2A multi-step orchestration, policy evaluation, and execution-plan registration.',
    version: runtime.config.version,
    protocol_version: '2026.a2a.v1',
    default_endpoint: `http://localhost:${runtime.config.port}/api/v1/brain`,
    capabilities: [
      {
        id: 'plan.reason',
        name: 'Plan and reason',
        description: 'Classify, decompose, disambiguate, and plan multi-step requests.',
        version: '1.0.0',
        input_modes: ['application/json'],
        output_modes: ['application/json'],
        async: true
      },
      {
        id: 'policy.evaluate',
        name: 'Policy evaluation',
        description: 'Compute privacy, approval, and execution policy metadata for candidate actions.',
        version: '1.0.0',
        input_modes: ['application/json'],
        output_modes: ['application/json'],
        async: true
      }
    ],
    supports: {
      task_lifecycle: false,
      task_query: false,
      artifact_updates: true,
      push_callbacks: false,
      capability_inventory: false
    }
  };
}

function buildRuntime(config?: BrainConfig): BrainRuntime {
  const resolvedConfig = config ?? loadBrainConfig();
  const disambiguationRules = loadDisambiguationRules(resolvedConfig.disambiguationConfigPath);
  const startedAtMs = Date.now();

  let runtimeRef: BrainRuntime | undefined;

  const server = http.createServer((req, res) => {
    const runtime = runtimeRef;
    if (!runtime) {
      sendJSON(res, 500, { error: 'runtime_not_ready' });
      return;
    }

    const ctx = requestContext(req);
    const method = req.method ?? 'GET';
    const path = new URL(req.url ?? '/', 'http://localhost').pathname;

    const onError = (statusCode: number, code: string, message?: string): void => {
      sendJSON(res, statusCode, message ? { error: code, message } : { error: code });
      logEvent(runtime, ctx, 'brain.request.error', 'WARN', {
        method,
        path,
        status_code: statusCode,
        code,
        message
      });
    };

    const parseRequest = async (): Promise<Record<string, unknown> | undefined> => {
      const rawBody = await readRawBody(req);
      const parsed = parseObject(rawBody);
      if (parsed.error) {
        onError(400, parsed.error);
        return undefined;
      }
      return parsed.value;
    };

    if (method === 'GET' && path === '/health') {
      sendJSON(res, 200, healthPayload(runtime, false));
      return;
    }

    if (method === 'GET' && path === '/health/deep') {
      try {
        authenticateRequest(runtime, req, ctx, 'admin');
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

    if (method === 'POST' && path === '/api/v1/brain/classify') {
      void (async () => {
        let identity: AuthenticatedIdentity;
        let callerContext: ReturnType<typeof callerContextFromRequest>;
        try {
          identity = authenticateRequest(runtime, req, ctx, 'api');
          callerContext = callerContextFromRequest(runtime, req);
        } catch (error) {
          onError(401, error instanceof Error ? error.message : 'authorization_required');
          return;
        }
        const payload = await parseRequest();
        if (!payload) {
          return;
        }
        const request = normalizePayload(payload, identity, callerContext);
        if (!request.message_text) {
          onError(400, 'message_text_required');
          return;
        }

        const output = classifyIntent(request as IntentClassificationInput);
        sendJSON(res, 200, output as unknown as Record<string, unknown>);
        logEvent(runtime, ctx, 'brain.classify.complete', 'INFO', {
          intent: output.intent,
          confidence: output.confidence,
          skills: output.skills,
          clarification_required: output.clarification_required
        });
      })().catch((err) => {
        onError(500, 'classify_failed');
        logEvent(runtime, ctx, 'brain.classify.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'POST' && path === '/api/v1/brain/disambiguate') {
      void (async () => {
        let identity: AuthenticatedIdentity;
        let callerContext: ReturnType<typeof callerContextFromRequest>;
        try {
          identity = authenticateRequest(runtime, req, ctx, 'api');
          callerContext = callerContextFromRequest(runtime, req);
        } catch (error) {
          onError(401, error instanceof Error ? error.message : 'authorization_required');
          return;
        }
        const payload = await parseRequest();
        if (!payload) {
          return;
        }
        const request = normalizePayload(payload, identity, callerContext);
        if (!request.message_text) {
          onError(400, 'message_text_required');
          return;
        }

        const disambiguationReq: DisambiguationRequest = {
          message_text: request.message_text,
          intent: asString(payload.intent),
          candidate_skills: asStringArray(payload.candidate_skills),
          deployment_mode: request.deployment_mode,
          user_tier: request.user_tier,
          user_preferences: request.user_preferences,
          enabled_skills: request.user_profile.enabled_skills,
          allow_multi_intent: asBool(payload.allow_multi_intent)
        };

        const output = disambiguateSkills(disambiguationReq, runtime.disambiguationRules);
        sendJSON(res, 200, output as unknown as Record<string, unknown>);
        logEvent(runtime, ctx, 'brain.disambiguate.complete', 'INFO', {
          group_hits: output.group_hits,
          resolved_skills: output.resolved_skills,
          clarification_required: output.clarification_required
        });
      })().catch((err) => {
        onError(500, 'disambiguation_failed');
        logEvent(runtime, ctx, 'brain.disambiguate.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'POST' && path === '/api/v1/brain/decompose') {
      void (async () => {
        try {
          authenticateRequest(runtime, req, ctx, 'api');
        } catch (error) {
          onError(401, error instanceof Error ? error.message : 'authorization_required');
          return;
        }
        const payload = await parseRequest();
        if (!payload) {
          return;
        }
        const requestText = asString(payload.request) ?? asString(payload.message_text);
        if (!requestText) {
          onError(400, 'message_text_required');
          return;
        }
        const skills = asStringArray(payload.skills) ?? [];
        const requires = asBool(payload.requires_decomposition) ?? requestText.toLowerCase().includes(' and ');

        const output = decomposeTask(requestText, skills, requires);
        sendJSON(res, 200, output as unknown as Record<string, unknown>);
        logEvent(runtime, ctx, 'brain.decompose.complete', 'INFO', {
          tasks: output.tasks.length,
          execution_order: output.execution_order
        });
      })().catch((err) => {
        if (err instanceof Error && err.message.startsWith('TASK_GRAPH_INVALID')) {
          sendJSON(res, 422, {
            error: 'TASK_GRAPH_INVALID',
            message: err.message
          });
          return;
        }
        onError(500, 'decompose_failed');
        logEvent(runtime, ctx, 'brain.decompose.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'POST' && path === '/api/v1/brain/aggregate') {
      void (async () => {
        let identity: AuthenticatedIdentity;
        let callerContext: ReturnType<typeof callerContextFromRequest>;
        try {
          identity = authenticateRequest(runtime, req, ctx, 'api');
          callerContext = callerContextFromRequest(runtime, req);
        } catch (error) {
          onError(401, error instanceof Error ? error.message : 'authorization_required');
          return;
        }
        const payload = await parseRequest();
        if (!payload) {
          return;
        }
        const normalized = normalizePayload(payload, identity, callerContext);
        const request: AggregationRequest = {
          skill_results: extractSkillResults(payload.skill_results),
          user_profile:
            payload.user_profile && typeof payload.user_profile === 'object' && !Array.isArray(payload.user_profile)
              ? (payload.user_profile as AggregationRequest['user_profile'])
              : undefined,
          channel: normalized.channel
        };

        const output = aggregateResults(request);
        sendJSON(res, 200, output as unknown as Record<string, unknown>);
        logEvent(runtime, ctx, 'brain.aggregate.complete', 'INFO', {
          suggested_actions: output.suggested_actions.length,
          completion_ratio: output.completion_ratio
        });
      })().catch((err) => {
        onError(500, 'aggregate_failed');
        logEvent(runtime, ctx, 'brain.aggregate.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'POST' && path === '/api/v1/brain/process') {
      void (async () => {
        let identity: AuthenticatedIdentity;
        let callerContext: ReturnType<typeof callerContextFromRequest>;
        try {
          identity = authenticateRequest(runtime, req, ctx, 'api');
          callerContext = callerContextFromRequest(runtime, req);
        } catch (error) {
          onError(401, error instanceof Error ? error.message : 'authorization_required');
          return;
        }
        const payload = await parseRequest();
        if (!payload) {
          return;
        }
        const request = normalizePayload(payload, identity, callerContext);
        if (!request.message_text) {
          onError(400, 'message_text_required');
          return;
        }

        const classification = classifyIntent(request);
        const { decomposition, disambiguation, plan } = await buildPlannerProposal(request, runtime.disambiguationRules, runtime.config);
        const verification = verifyPlan(plan, request.skill_results, request);
        const aggregation = request.skill_results && request.skill_results.length > 0
          ? aggregateResults({
              skill_results: request.skill_results,
              user_profile: {
                communication_style: request.user_profile.communication_style
              },
              channel: request.channel
            })
          : undefined;

        const executionStatus = !verification.valid
          ? 'verification_failed'
          : plan.requires_clarification
            ? 'clarification_required'
            : aggregation
              ? 'completed'
              : 'dispatch_ready';

        const planRegistration = verification.valid
          ? await registerExecutionPlan(plan.run_id, plan, runtime.config)
          : { delegated: false, registeredSteps: 0, warning: 'verification_blocked_execution_plan_registration' };
        if (planRegistration.warning) {
          logEvent(runtime, ctx, 'brain.process.workflow_plan_registration_skipped', 'WARN', {
            run_id: plan.run_id,
            warning: planRegistration.warning
          });
        }

        const verificationAnnotation = !verification.valid
          ? await annotateRunVerification(plan.run_id, verification, runtime.config)
          : { delegated: false, warning: undefined };
        if (verificationAnnotation.warning) {
          logEvent(runtime, ctx, 'brain.process.workflow_verification_annotation_skipped', 'WARN', {
            run_id: plan.run_id,
            warning: verificationAnnotation.warning
          });
        }

        const workflowSync = await syncProcessRunState(plan.run_id, executionStatus, runtime.config);
        if (workflowSync.warning) {
          logEvent(runtime, ctx, 'brain.process.workflow_sync_skipped', 'WARN', {
            run_id: plan.run_id,
            warning: workflowSync.warning
          });
        }

        sendJSON(res, 200, {
          run_id: plan.run_id,
          thread_id: plan.thread_id,
          classification,
          disambiguation,
          decomposition,
          plan,
          verification,
          aggregation,
          execution_status: executionStatus
        });

        logEvent(runtime, ctx, 'brain.process.complete', 'INFO', {
          intent: classification.intent,
          resolved_skills: disambiguation.resolved_skills,
          tasks: decomposition.tasks.length,
          planner_provider: plan.planner_provider,
          planner_mode: plan.planner_mode,
          execution_status: executionStatus,
          workflow_plan_registered: planRegistration.delegated,
          workflow_registered_steps: planRegistration.registeredSteps,
          workflow_verification_annotated: verificationAnnotation.delegated,
          workflow_runtime_delegated: workflowSync.delegated,
          workflow_transitions: workflowSync.transitioned
        });
      })().catch((err) => {
        if (err instanceof Error && err.message.startsWith('TASK_GRAPH_INVALID')) {
          sendJSON(res, 422, {
            error: 'TASK_GRAPH_INVALID',
            message: err.message
          });
          return;
        }
        onError(500, 'process_failed');
        logEvent(runtime, ctx, 'brain.process.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    onError(404, 'not_found');
  });

  const runtime: BrainRuntime = {
    config: resolvedConfig,
    startedAtMs,
    disambiguationRules,
    server,
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

function installSignalHandlers(runtime: BrainRuntime): void {
  const shutdown = async (signal: string): Promise<void> => {
    const ctx: RequestContext = {
      traceId: randomUUID(),
      spanId: randomUUID(),
      requestId: randomUUID()
    };

    logEvent(runtime, ctx, 'brain.shutdown.start', 'INFO', { signal });

    const timeout = setTimeout(() => {
      logEvent(runtime, ctx, 'brain.shutdown.timeout', 'ERROR', {
        timeout_ms: runtime.config.shutdownTimeoutMs
      });
      process.exit(1);
    }, runtime.config.shutdownTimeoutMs);

    try {
      await runtime.close();
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'brain.shutdown.complete', 'INFO', {});
      process.exit(0);
    } catch (err) {
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'brain.shutdown.failed', 'ERROR', {
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

  logEvent(runtime, ctx, 'brain.started', 'INFO', {
    port: runtime.config.port,
    disambiguation_rules: Object.keys(runtime.disambiguationRules).length,
    disambiguation_config: runtime.config.disambiguationConfigPath,
    planner_provider: runtime.config.plannerProvider,
    planner_model: runtime.config.plannerModel
  });
}

if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url) {
  void main().catch((err) => {
    process.stderr.write(
      JSON.stringify({
        ts: new Date().toISOString(),
        service: 'brevio-brain',
        event: 'brain.start.failed',
        severity: 'ERROR',
        message: err instanceof Error ? err.message : String(err)
      }) + '\n'
    );
    process.exit(1);
  });
}

export { buildRuntime as createBrainRuntime };
