import { randomUUID } from 'node:crypto';
import http from 'node:http';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

import {
  authenticateInternalRequest,
  resolveEffectiveUserScope
} from '../../../packages/shared/src/internal-http-auth.js';
import {
  buildAccessTokenIssuerRegistry,
  buildCallerContextIssuerRegistry,
  loadBrevioEnvironment,
  resolveAccessTokenVerificationKey,
  resolveCallerContextVerificationKey,
  type AccessTokenIssuerRegistry,
  type CallerContextIssuerRegistry
} from '../../../packages/shared/src/security.js';
import { SchedulerStore, type ScheduledJob, type TriggerEvent } from './scheduler-store.js';

interface SchedulerConfig {
  serviceName: string;
  version: string;
  environment: string;
  port: number;
  shutdownTimeoutMs: number;
  maxBodyBytes: number;
  maxJobs: number;
  stateFilePath?: string;
  accessTokenIssuers: AccessTokenIssuerRegistry;
  serviceAudience: string;
  callerContextIssuers: CallerContextIssuerRegistry;
  logSalt: string;
}

interface RequestContext {
  traceId: string;
  spanId: string;
  requestId: string;
  subjectRef?: string;
}

interface SchedulerRuntime {
  config: SchedulerConfig;
  startedAtMs: number;
  server: http.Server;
  store: SchedulerStore;
  close(): Promise<void>;
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

function loadConfig(): SchedulerConfig {
  const environment = loadBrevioEnvironment();
  return {
    serviceName: 'brevio-scheduler',
    version: process.env.SERVICE_VERSION ?? '0.2.0',
    environment,
    port: parsePositiveInt(process.env.PORT, 8085, 'PORT'),
    shutdownTimeoutMs: parsePositiveInt(process.env.BREVIO_SCHEDULER_SHUTDOWN_TIMEOUT_MS, 30000, 'BREVIO_SCHEDULER_SHUTDOWN_TIMEOUT_MS'),
    maxBodyBytes: parsePositiveInt(process.env.BREVIO_SCHEDULER_MAX_BODY_BYTES, 256 * 1024, 'BREVIO_SCHEDULER_MAX_BODY_BYTES'),
    maxJobs: parsePositiveInt(process.env.BREVIO_SCHEDULER_MAX_JOBS, 5000, 'BREVIO_SCHEDULER_MAX_JOBS'),
    stateFilePath: path.resolve(process.env.BREVIO_SCHEDULER_STATE_FILE ?? path.join(process.cwd(), 'data', 'scheduler', 'state.json')),
    accessTokenIssuers: buildAccessTokenIssuerRegistry([
      {
        issuer: process.env.BREVIO_AUTH_ACCESS_ISSUER?.trim() || 'https://auth.brevio.internal',
        verificationKey: resolveAccessTokenVerificationKey(
          process.env.BREVIO_AUTH_ACCESS_PUBLIC_KEY,
          undefined,
          undefined,
          environment,
          'BREVIO_AUTH_ACCESS_PUBLIC_KEY',
          'auth-access'
        ),
        allowedTokenUses: ['user_access', 'admin_access']
      },
      {
        issuer: process.env.BREVIO_GATEWAY_SERVICE_ISSUER?.trim() || 'https://gateway.brevio.internal',
        verificationKey: resolveAccessTokenVerificationKey(
          process.env.BREVIO_GATEWAY_SERVICE_PUBLIC_KEY,
          undefined,
          undefined,
          environment,
          'BREVIO_GATEWAY_SERVICE_PUBLIC_KEY',
          'gateway-service'
        ),
        allowedTokenUses: ['service_access']
      }
    ]),
    serviceAudience: process.env.BREVIO_SCHEDULER_AUDIENCE?.trim() || 'brevio-scheduler',
    callerContextIssuers: buildCallerContextIssuerRegistry([
      {
        issuer: process.env.BREVIO_GATEWAY_CALLER_CONTEXT_ISSUER?.trim() || 'https://gateway.brevio.internal/caller-context',
        verificationKey: resolveCallerContextVerificationKey(
          process.env.BREVIO_GATEWAY_CALLER_CONTEXT_PUBLIC_KEY,
          environment,
          'BREVIO_GATEWAY_CALLER_CONTEXT_PUBLIC_KEY',
          'gateway-caller-context'
        )
      }
    ]),
    logSalt: process.env.BREVIO_SCHEDULER_LOG_SALT?.trim() || `brevio-scheduler:${environment}`
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
  runtime: SchedulerRuntime,
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

function sanitizeUserId(raw: string | undefined): string | undefined {
  if (!raw) {
    return undefined;
  }
  const trimmed = raw.trim();
  if (/^[a-zA-Z0-9-]{8,64}$/.test(trimmed)) {
    return trimmed;
  }
  return undefined;
}

function healthPayload(runtime: SchedulerRuntime, deep: boolean): Record<string, unknown> {
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
      persistence_mode: runtime.store.mode(),
      durable_schedules: runtime.store.mode() !== 'in_memory',
      shared_schedule_backend: false
    },
    scheduler: {
      jobs: runtime.store.stats().jobs,
      queued_triggers: runtime.store.stats().queuedTriggers
    }
  };
}

function normalizeJob(job: ScheduledJob): Record<string, unknown> {
  return {
    id: job.id,
    user_id: job.user_id,
    skill_id: job.skill_id,
    schedule: job.schedule,
    timezone: job.timezone,
    status: job.status,
    payload: job.payload,
    last_run_at: job.last_run_at,
    next_run_at: job.next_run_at,
    created_at: job.created_at,
    updated_at: job.updated_at
  };
}

function createJob(runtime: SchedulerRuntime, payload: Record<string, unknown>, userId: string): ScheduledJob {
  if (runtime.store.jobCount() >= runtime.config.maxJobs) {
    throw new Error('job_limit_exceeded');
  }

  const skillId = asString(payload.skill_id);
  const schedule = asString(payload.schedule);

  if (!skillId) {
    throw new Error('skill_id_required');
  }
  if (!schedule) {
    throw new Error('schedule_required');
  }

  const now = new Date().toISOString();
  const job: ScheduledJob = {
    id: randomUUID(),
    user_id: userId,
    skill_id: skillId,
    schedule,
    timezone: asString(payload.timezone) ?? 'UTC',
    status: 'active',
    payload: asObject(payload.payload) ?? {},
    next_run_at: now,
    created_at: now,
    updated_at: now
  };

  return runtime.store.saveJob(job);
}

function createTrigger(runtime: SchedulerRuntime, payload: Record<string, unknown>, userId: string): TriggerEvent {
  const skillId = asString(payload.skill_id);

  if (!skillId) {
    throw new Error('skill_id_required');
  }

  const trigger: TriggerEvent = {
    id: randomUUID(),
    user_id: userId,
    skill_id: skillId,
    payload: asObject(payload.payload) ?? {},
    status: 'queued',
    created_at: new Date().toISOString()
  };

  return runtime.store.appendTrigger(trigger, 500);
}

function runJob(runtime: SchedulerRuntime, job: ScheduledJob): TriggerEvent {
  job.last_run_at = new Date().toISOString();
  job.next_run_at = new Date(Date.now() + 60_000).toISOString();
  job.updated_at = new Date().toISOString();
  runtime.store.saveJob(job);

  const trigger: TriggerEvent = {
    id: randomUUID(),
    user_id: job.user_id,
    skill_id: job.skill_id,
    payload: job.payload,
    status: 'queued',
    created_at: new Date().toISOString()
  };

  return runtime.store.appendTrigger(trigger, 500);
}

function buildRuntime(config?: SchedulerConfig): SchedulerRuntime {
  const resolvedConfig = config ?? loadConfig();
  const startedAtMs = Date.now();
  const store = new SchedulerStore(resolvedConfig.stateFilePath);

  let runtimeRef: SchedulerRuntime | undefined;

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
      logEvent(runtime, ctx, 'scheduler.request.error', 'WARN', {
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
    if (!segments || segments[0] !== 'scheduler') {
      onError(404, 'not_found');
      return;
    }

    let auth: ReturnType<typeof authenticateInternalRequest>;
    try {
      auth = authenticateInternalRequest(req, runtime.config, ctx);
    } catch (error) {
      onError(401, error instanceof Error ? error.message : 'authorization_required');
      return;
    }

    const effectiveScope = resolveEffectiveUserScope(auth);
    const requestUserId = effectiveScope.userId;

    if (method === 'GET' && segments.length === 2 && segments[1] === 'jobs') {
      const jobs = runtime.store
        .listJobs()
        .filter((job) => auth.principal.token_use !== 'user_access' || job.user_id === requestUserId)
        .map((job) => normalizeJob(job));
      sendJSON(res, 200, {
        total: jobs.length,
        jobs
      });
      return;
    }

    if (method === 'POST' && segments.length === 2 && segments[1] === 'jobs') {
      void (async () => {
        const rawBody = await readRawBody(req, runtime.config.maxBodyBytes);
        const payload = parseObject(rawBody);
        const userId = sanitizeUserId(requestUserId);
        if (!userId) {
          throw new Error('caller_context_required');
        }
        const job = createJob(runtime, payload, userId);

        sendJSON(res, 201, {
          job: normalizeJob(job)
        });

        logEvent(runtime, ctx, 'scheduler.job.created', 'INFO', {
          job_id: job.id,
          user_id: job.user_id,
          skill_id: job.skill_id
        });
      })().catch((err) => {
        const code = err instanceof Error ? err.message : 'job_create_failed';
        switch (code) {
          case 'payload_too_large':
            onError(413, code);
            return;
          case 'invalid_json':
          case 'caller_context_required':
          case 'skill_id_required':
          case 'schedule_required':
            onError(400, code);
            return;
          case 'job_limit_exceeded':
            onError(429, code);
            return;
          default:
            onError(500, 'job_create_failed');
            logEvent(runtime, ctx, 'scheduler.job.create.exception', 'ERROR', {
              message: err instanceof Error ? err.message : String(err)
            });
        }
      });
      return;
    }

    if (method === 'POST' && segments.length === 4 && segments[1] === 'jobs' && segments[3] === 'run') {
      const job = runtime.store.getJob(segments[2]);
      if (!job) {
        onError(404, 'job_not_found');
        return;
      }
      if (auth.principal.token_use === 'user_access' && job.user_id !== requestUserId) {
        onError(403, 'forbidden');
        return;
      }
      if (job.status !== 'active') {
        onError(409, 'job_not_active');
        return;
      }

      const trigger = runJob(runtime, job);
      sendJSON(res, 200, {
        job: normalizeJob(job),
        trigger
      });

      logEvent(runtime, ctx, 'scheduler.job.run', 'INFO', {
        job_id: job.id,
        trigger_id: trigger.id
      });
      return;
    }

    if (method === 'DELETE' && segments.length === 3 && segments[1] === 'jobs') {
      const job = runtime.store.getJob(segments[2]);
      if (!job) {
        onError(404, 'job_not_found');
        return;
      }
      if (auth.principal.token_use === 'user_access' && job.user_id !== requestUserId) {
        onError(403, 'forbidden');
        return;
      }

      job.status = 'disabled';
      job.updated_at = new Date().toISOString();
      runtime.store.saveJob(job);

      sendJSON(res, 200, {
        job: normalizeJob(job)
      });

      logEvent(runtime, ctx, 'scheduler.job.disabled', 'INFO', {
        job_id: job.id
      });
      return;
    }

    if (method === 'POST' && segments.length === 2 && segments[1] === 'trigger') {
      void (async () => {
        const rawBody = await readRawBody(req, runtime.config.maxBodyBytes);
        const payload = parseObject(rawBody);
        const userId = sanitizeUserId(requestUserId);
        if (!userId) {
          throw new Error('caller_context_required');
        }
        const trigger = createTrigger(runtime, payload, userId);

        sendJSON(res, 202, { trigger });

        logEvent(runtime, ctx, 'scheduler.trigger.queued', 'INFO', {
          trigger_id: trigger.id,
          user_id: trigger.user_id,
          skill_id: trigger.skill_id
        });
      })().catch((err) => {
        const code = err instanceof Error ? err.message : 'trigger_queue_failed';
        switch (code) {
          case 'payload_too_large':
            onError(413, code);
            return;
          case 'invalid_json':
          case 'caller_context_required':
          case 'skill_id_required':
            onError(400, code);
            return;
          default:
            onError(500, 'trigger_queue_failed');
            logEvent(runtime, ctx, 'scheduler.trigger.exception', 'ERROR', {
              message: err instanceof Error ? err.message : String(err)
            });
        }
      });
      return;
    }

    if (method === 'GET' && segments.length === 2 && segments[1] === 'triggers') {
      const triggers = runtime.store
        .listTriggers()
        .filter((trigger) => auth.principal.token_use !== 'user_access' || trigger.user_id === requestUserId);
      sendJSON(res, 200, {
        total: triggers.length,
        triggers
      });
      return;
    }

    onError(404, 'not_found');
  });

  const runtime: SchedulerRuntime = {
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

function installSignalHandlers(runtime: SchedulerRuntime): void {
  const shutdown = async (signal: string): Promise<void> => {
    const ctx: RequestContext = {
      traceId: randomUUID(),
      spanId: randomUUID(),
      requestId: randomUUID()
    };

    logEvent(runtime, ctx, 'scheduler.shutdown.start', 'INFO', { signal });

    const timeout = setTimeout(() => {
      logEvent(runtime, ctx, 'scheduler.shutdown.timeout', 'ERROR', {
        timeout_ms: runtime.config.shutdownTimeoutMs
      });
      process.exit(1);
    }, runtime.config.shutdownTimeoutMs);

    try {
      await runtime.close();
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'scheduler.shutdown.complete', 'INFO', {});
      process.exit(0);
    } catch (err) {
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'scheduler.shutdown.failed', 'ERROR', {
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

  logEvent(runtime, ctx, 'scheduler.started', 'INFO', {
    port: runtime.config.port,
    max_jobs: runtime.config.maxJobs
  });
}

if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url) {
  void main().catch((err) => {
    process.stderr.write(
      JSON.stringify({
        ts: new Date().toISOString(),
        service: 'brevio-scheduler',
        event: 'scheduler.start.failed',
        severity: 'ERROR',
        message: err instanceof Error ? err.message : String(err)
      }) + '\n'
    );
    process.exit(1);
  });
}

export { buildRuntime as createSchedulerRuntime };
