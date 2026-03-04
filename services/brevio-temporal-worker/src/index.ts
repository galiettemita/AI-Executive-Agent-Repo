import { randomUUID } from 'node:crypto';
import http from 'node:http';

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

type WorkflowStatus = 'RUNNING' | 'COMPLETED' | 'FAILED' | 'DEAD_LETTER';

interface WorkerConfig {
  serviceName: string;
  version: string;
  environment: string;
  port: number;
  shutdownTimeoutMs: number;
  maxBodyBytes: number;
}

interface RequestContext {
  traceId: string;
  spanId: string;
  requestId: string;
  userId?: string;
}

interface WorkflowRun {
  run_id: string;
  workflow_id: string;
  workflow_type: 'message-processing' | 'daily-rhythm';
  user_id?: string;
  status: WorkflowStatus;
  states: string[];
  current_state: string;
  started_at: string;
  completed_at?: string;
  metadata: Record<string, unknown>;
}

interface WorkerRuntime {
  config: WorkerConfig;
  startedAtMs: number;
  server: http.Server;
  runs: Map<string, WorkflowRun>;
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

function loadConfig(): WorkerConfig {
  return {
    serviceName: 'brevio-temporal-worker',
    version: process.env.SERVICE_VERSION ?? '0.2.0',
    environment: process.env.NODE_ENV ?? 'development',
    port: parsePositiveInt(process.env.PORT, 8087, 'PORT'),
    shutdownTimeoutMs: parsePositiveInt(process.env.BREVIO_TEMPORAL_WORKER_SHUTDOWN_TIMEOUT_MS, 30000, 'BREVIO_TEMPORAL_WORKER_SHUTDOWN_TIMEOUT_MS'),
    maxBodyBytes: parsePositiveInt(process.env.BREVIO_TEMPORAL_WORKER_MAX_BODY_BYTES, 256 * 1024, 'BREVIO_TEMPORAL_WORKER_MAX_BODY_BYTES')
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

function buildMessageWorkflowStates(failureState?: MessageWorkflowState): { states: MessageWorkflowState[]; status: WorkflowStatus } {
  const ordered: MessageWorkflowState[] = [
    'RECEIVED',
    'CLASSIFYING',
    'DECOMPOSING',
    'EXECUTING',
    'AGGREGATING',
    'FORMATTING',
    'DELIVERING',
    'COMPLETED'
  ];

  if (!failureState || failureState === 'COMPLETED') {
    return {
      states: ordered,
      status: 'COMPLETED'
    };
  }

  const idx = ordered.indexOf(failureState);
  if (idx < 0) {
    return {
      states: [...ordered.slice(0, ordered.length - 1), 'FAILED'],
      status: 'FAILED'
    };
  }

  const failedStates = [...ordered.slice(0, idx + 1), failureState === 'RECEIVED' ? 'DEAD_LETTER' : 'FAILED'];
  return {
    states: failedStates,
    status: failureState === 'RECEIVED' ? 'DEAD_LETTER' : 'FAILED'
  };
}

function buildDailyWorkflowStates(): DailyWorkflowState[] {
  return ['INIT', 'COMPOSING', 'DELIVERING', 'COMPLETED'];
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
      db: process.env.DATABASE_URL ? 'configured' : 'not_configured',
      redis: process.env.REDIS_URL ? 'configured' : 'not_configured',
      temporal: process.env.TEMPORAL_HOST ? 'configured' : 'not_configured'
    },
    workflow_runs: runtime.runs.size
  };
}

function buildRuntime(config?: WorkerConfig): WorkerRuntime {
  const resolvedConfig = config ?? loadConfig();
  const startedAtMs = Date.now();

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

    if (method === 'GET' && pathname === '/health/deep') {
      sendJSON(res, 200, healthPayload(runtime, true));
      return;
    }

    const segments = parseApiPath(pathname);
    if (!segments || segments[0] !== 'temporal-worker') {
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

    if (method === 'GET' && segments.length === 3 && segments[1] === 'runs') {
      const run = runtime.runs.get(segments[2]);
      if (!run) {
        onError(404, 'run_not_found');
        return;
      }
      sendJSON(res, 200, run as unknown as Record<string, unknown>);
      return;
    }

    if (method === 'POST' && segments.length === 3 && segments[1] === 'workflows' && segments[2] === 'message-processing') {
      void (async () => {
        const rawBody = await readRawBody(req, runtime.config.maxBodyBytes);
        const payload = parseObject(rawBody);

        const messageId = asString(payload.message_id);
        if (!messageId) {
          onError(400, 'message_id_required');
          return;
        }

        const workflowId = `msg-${messageId}`;
        const runId = randomUUID();
        const failState = asString(payload.force_fail_state) as MessageWorkflowState | undefined;
        const plan = buildMessageWorkflowStates(failState);
        const now = new Date().toISOString();

        const run: WorkflowRun = {
          run_id: runId,
          workflow_id: workflowId,
          workflow_type: 'message-processing',
          user_id: asString(payload.user_id),
          status: plan.status,
          states: plan.states,
          current_state: plan.states[plan.states.length - 1],
          started_at: now,
          completed_at: now,
          metadata: {
            message_id: messageId,
            deterministic_jitter_ms: deterministicJitterMs(runId, 1, 500)
          }
        };

        runtime.runs.set(run.run_id, run);

        sendJSON(res, 202, run as unknown as Record<string, unknown>);
        logEvent(runtime, ctx, 'temporal_worker.message_processing.started', 'INFO', {
          workflow_id: workflowId,
          run_id: runId,
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

        const userId = asString(payload.user_id);
        if (!userId) {
          onError(400, 'user_id_required');
          return;
        }

        const runId = randomUUID();
        const workflowId = `daily-rhythm-${userId}-${new Date().toISOString().slice(0, 10)}`;
        const states = buildDailyWorkflowStates();
        const now = new Date().toISOString();

        const run: WorkflowRun = {
          run_id: runId,
          workflow_id: workflowId,
          workflow_type: 'daily-rhythm',
          user_id: userId,
          status: 'COMPLETED',
          states,
          current_state: 'COMPLETED',
          started_at: now,
          completed_at: now,
          metadata: {
            wake_time: asString(payload.wake_time) ?? '07:00',
            deterministic_jitter_ms: deterministicJitterMs(runId, 1, 1000)
          }
        };

        runtime.runs.set(run.run_id, run);

        sendJSON(res, 202, run as unknown as Record<string, unknown>);
        logEvent(runtime, ctx, 'temporal_worker.daily_rhythm.started', 'INFO', {
          workflow_id: workflowId,
          run_id: runId
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
    runs: new Map(),
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
    workflows: ['message-processing', 'daily-rhythm']
  });
}

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

export { buildRuntime as createTemporalWorkerRuntime };
