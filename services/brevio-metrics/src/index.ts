import { randomUUID } from 'node:crypto';
import http from 'node:http';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

import { authenticateInternalRequest } from '../../../packages/shared/src/internal-http-auth.js';
import {
  buildAccessTokenIssuerRegistry,
  loadBrevioEnvironment,
  resolveAccessTokenVerificationKey,
  type AccessTokenIssuerRegistry
} from '../../../packages/shared/src/security.js';
import { MetricsStore } from './metrics-store.js';

interface MetricsConfig {
  serviceName: string;
  version: string;
  environment: string;
  port: number;
  shutdownTimeoutMs: number;
  maxBodyBytes: number;
  stateFilePath?: string;
  accessTokenIssuers: AccessTokenIssuerRegistry;
  serviceAudience: string;
  logSalt: string;
}

interface RequestContext {
  traceId: string;
  spanId: string;
  requestId: string;
  subjectRef?: string;
}

type MetricType = 'counter' | 'gauge' | 'histogram';

interface MetricDescriptor {
  name: string;
  type: MetricType;
  help: string;
}

interface MetricsRuntime {
  config: MetricsConfig;
  startedAtMs: number;
  server: http.Server;
  store: MetricsStore;
  close(): Promise<void>;
}

const LATENCY_BUCKETS = [50, 100, 250, 500, 1000, 2500, 5000, 10000];

const METRICS: MetricDescriptor[] = [
  { name: 'brevio_messages_total', type: 'counter', help: 'Total messages by channel direction and status.' },
  { name: 'brevio_message_latency_ms', type: 'histogram', help: 'Message processing latency in milliseconds.' },
  { name: 'brevio_skill_executions_total', type: 'counter', help: 'Total skill executions by skill_id status and cache_hit.' },
  { name: 'brevio_skill_latency_ms', type: 'histogram', help: 'Skill execution latency in milliseconds.' },
  { name: 'brevio_llm_tokens_total', type: 'counter', help: 'Total LLM tokens consumed.' },
  { name: 'brevio_llm_cost_cents', type: 'counter', help: 'Total LLM cost in cents.' },
  { name: 'brevio_circuit_breaker_state', type: 'gauge', help: 'Circuit breaker state per skill (0 closed, 1 half-open, 2 open).' },
  { name: 'brevio_active_sessions', type: 'gauge', help: 'Current active sessions by channel.' },
  { name: 'brevio_auth_token_refreshes', type: 'counter', help: 'OAuth token refresh attempts by service and status.' },
  { name: 'brevio_budget_utilization_pct', type: 'gauge', help: 'Budget utilization percentage by user tier.' }
];

const METRIC_BY_NAME = new Map<string, MetricDescriptor>(METRICS.map((metric) => [metric.name, metric]));

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

function loadConfig(): MetricsConfig {
  const environment = loadBrevioEnvironment();
  return {
    serviceName: 'brevio-metrics',
    version: process.env.SERVICE_VERSION ?? '0.2.0',
    environment,
    port: parsePositiveInt(process.env.PORT, 9090, 'PORT'),
    shutdownTimeoutMs: parsePositiveInt(process.env.BREVIO_METRICS_SHUTDOWN_TIMEOUT_MS, 30000, 'BREVIO_METRICS_SHUTDOWN_TIMEOUT_MS'),
    maxBodyBytes: parsePositiveInt(process.env.BREVIO_METRICS_MAX_BODY_BYTES, 128 * 1024, 'BREVIO_METRICS_MAX_BODY_BYTES'),
    stateFilePath: path.resolve(process.env.BREVIO_METRICS_STATE_FILE ?? path.join(process.cwd(), 'data', 'metrics', 'state.json')),
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
    serviceAudience: process.env.BREVIO_METRICS_AUDIENCE?.trim() || 'brevio-metrics',
    logSalt: process.env.BREVIO_METRICS_LOG_SALT?.trim() || `brevio-metrics:${environment}`
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
  runtime: MetricsRuntime,
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

function asNumber(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
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

function normalizeLabels(value: unknown): Record<string, string> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return {};
  }
  const labels: Record<string, string> = {};
  for (const [key, raw] of Object.entries(value)) {
    const normalizedKey = key.trim();
    if (!/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(normalizedKey)) {
      continue;
    }
    if (typeof raw === 'string' || typeof raw === 'number' || typeof raw === 'boolean') {
      labels[normalizedKey] = String(raw);
    }
  }
  return labels;
}

function labelKey(labels: Record<string, string>): string {
  const entries = Object.entries(labels).sort(([a], [b]) => a.localeCompare(b));
  return entries.map(([key, value]) => `${key}=${value}`).join('|');
}

function parseSeriesKey(seriesKey: string): Record<string, string> {
  if (seriesKey === '') {
    return {};
  }
  const labels: Record<string, string> = {};
  for (const token of seriesKey.split('|')) {
    const idx = token.indexOf('=');
    if (idx <= 0) {
      continue;
    }
    labels[token.slice(0, idx)] = token.slice(idx + 1);
  }
  return labels;
}

function labelsToPrometheus(labels: Record<string, string>): string {
  const entries = Object.entries(labels).sort(([a], [b]) => a.localeCompare(b));
  if (entries.length === 0) {
    return '';
  }
  const encoded = entries.map(([key, value]) => `${key}="${value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`).join(',');
  return `{${encoded}}`;
}

function upsertMetric(runtime: MetricsRuntime, payload: Record<string, unknown>): { metric: string; type: MetricType; labels: Record<string, string> } {
  const metricName = asString(payload.metric);
  if (!metricName) {
    throw new Error('metric_required');
  }

  const descriptor = METRIC_BY_NAME.get(metricName);
  if (!descriptor) {
    throw new Error('unknown_metric');
  }

  const labels = normalizeLabels(payload.labels);
  const key = `${metricName}::${labelKey(labels)}`;

  if (descriptor.type === 'counter') {
    const delta = asNumber(payload.value) ?? 1;
    if (delta < 0) {
      throw new Error('counter_negative_increment');
    }
    runtime.store.incrementCounter(key, delta);
  } else if (descriptor.type === 'gauge') {
    const value = asNumber(payload.value);
    if (typeof value !== 'number') {
      throw new Error('gauge_value_required');
    }
    runtime.store.setGauge(key, value);
  } else {
    const observation = asNumber(payload.value);
    if (typeof observation !== 'number') {
      throw new Error('histogram_value_required');
    }
    runtime.store.observeHistogram(key, observation, LATENCY_BUCKETS);
  }

  return {
    metric: descriptor.name,
    type: descriptor.type,
    labels
  };
}

function renderPrometheus(runtime: MetricsRuntime): string {
  const lines: string[] = [];

  for (const descriptor of METRICS) {
    lines.push(`# HELP ${descriptor.name} ${descriptor.help}`);
    lines.push(`# TYPE ${descriptor.name} ${descriptor.type}`);

    if (descriptor.type === 'counter') {
      const prefix = `${descriptor.name}::`;
      for (const [key, value] of runtime.store.counterEntries()) {
        if (!key.startsWith(prefix)) {
          continue;
        }
        const labels = parseSeriesKey(key.slice(prefix.length));
        lines.push(`${descriptor.name}${labelsToPrometheus(labels)} ${value}`);
      }
      continue;
    }

    if (descriptor.type === 'gauge') {
      const prefix = `${descriptor.name}::`;
      for (const [key, value] of runtime.store.gaugeEntries()) {
        if (!key.startsWith(prefix)) {
          continue;
        }
        const labels = parseSeriesKey(key.slice(prefix.length));
        lines.push(`${descriptor.name}${labelsToPrometheus(labels)} ${value}`);
      }
      continue;
    }

    const prefix = `${descriptor.name}::`;
    for (const [key, series] of runtime.store.histogramEntries()) {
      if (!key.startsWith(prefix)) {
        continue;
      }
      const labels = parseSeriesKey(key.slice(prefix.length));
      for (const bucket of LATENCY_BUCKETS) {
        const bucketLabels = {
          ...labels,
          le: String(bucket)
        };
        lines.push(`${descriptor.name}_bucket${labelsToPrometheus(bucketLabels)} ${series.buckets.get(bucket) ?? 0}`);
      }
      lines.push(`${descriptor.name}_bucket${labelsToPrometheus({ ...labels, le: '+Inf' })} ${series.count}`);
      lines.push(`${descriptor.name}_sum${labelsToPrometheus(labels)} ${series.sum}`);
      lines.push(`${descriptor.name}_count${labelsToPrometheus(labels)} ${series.count}`);
    }
  }

  return `${lines.join('\n')}\n`;
}

function metricsSnapshot(runtime: MetricsRuntime): Record<string, unknown> {
  return runtime.store.snapshot();
}

function healthPayload(runtime: MetricsRuntime, deep: boolean): Record<string, unknown> {
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
      storage_mode: runtime.store.mode(),
      durable_metrics_backend: runtime.store.mode() !== 'in_memory',
      shared_metrics_backend: false
    },
    metric_series: {
      counters: runtime.store.stats().counters,
      gauges: runtime.store.stats().gauges,
      histograms: runtime.store.stats().histograms
    }
  };
}

function buildRuntime(config?: MetricsConfig): MetricsRuntime {
  const resolvedConfig = config ?? loadConfig();
  const startedAtMs = Date.now();
  const store = new MetricsStore(resolvedConfig.stateFilePath);

  let runtimeRef: MetricsRuntime | undefined;

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
      logEvent(runtime, ctx, 'metrics.request.error', 'WARN', {
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

    if (method === 'GET' && pathname === '/metrics') {
      res.writeHead(200, { 'content-type': 'text/plain; version=0.0.4; charset=utf-8' });
      res.end(renderPrometheus(runtime));
      return;
    }

    const segments = parseApiPath(pathname);
    if (!segments || segments[0] !== 'metrics') {
      onError(404, 'not_found');
      return;
    }
    try {
      authenticateInternalRequest(req, runtime.config, ctx, {
        allowedTokenUses: ['service_access', 'admin_access']
      });
    } catch (error) {
      onError(401, error instanceof Error ? error.message : 'authorization_required');
      return;
    }

    if (method === 'POST' && segments.length === 2 && segments[1] === 'events') {
      void (async () => {
        const rawBody = await readRawBody(req, runtime.config.maxBodyBytes);
        const payload = parseObject(rawBody);
        const updated = upsertMetric(runtime, payload);

        sendJSON(res, 202, {
          metric: updated.metric,
          type: updated.type,
          labels: updated.labels
        });

        logEvent(runtime, ctx, 'metrics.event.ingested', 'INFO', {
          metric: updated.metric,
          type: updated.type,
          labels: updated.labels
        });
      })().catch((err) => {
        const code = err instanceof Error ? err.message : 'event_ingest_failed';
        switch (code) {
          case 'payload_too_large':
            onError(413, code);
            return;
          case 'invalid_json':
          case 'metric_required':
          case 'unknown_metric':
          case 'counter_negative_increment':
          case 'gauge_value_required':
          case 'histogram_value_required':
            onError(400, code);
            return;
          default:
            onError(500, 'event_ingest_failed');
            logEvent(runtime, ctx, 'metrics.event.exception', 'ERROR', {
              message: err instanceof Error ? err.message : String(err)
            });
        }
      });
      return;
    }

    if (method === 'GET' && segments.length === 2 && segments[1] === 'snapshot') {
      sendJSON(res, 200, metricsSnapshot(runtime));
      return;
    }

    onError(404, 'not_found');
  });

  const runtime: MetricsRuntime = {
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

function installSignalHandlers(runtime: MetricsRuntime): void {
  const shutdown = async (signal: string): Promise<void> => {
    const ctx: RequestContext = {
      traceId: randomUUID(),
      spanId: randomUUID(),
      requestId: randomUUID()
    };

    logEvent(runtime, ctx, 'metrics.shutdown.start', 'INFO', { signal });

    const timeout = setTimeout(() => {
      logEvent(runtime, ctx, 'metrics.shutdown.timeout', 'ERROR', {
        timeout_ms: runtime.config.shutdownTimeoutMs
      });
      process.exit(1);
    }, runtime.config.shutdownTimeoutMs);

    try {
      await runtime.close();
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'metrics.shutdown.complete', 'INFO', {});
      process.exit(0);
    } catch (err) {
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'metrics.shutdown.failed', 'ERROR', {
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

  logEvent(runtime, ctx, 'metrics.started', 'INFO', {
    port: runtime.config.port,
    metric_definitions: METRICS.length
  });
}

if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url) {
  void main().catch((err) => {
    process.stderr.write(
      JSON.stringify({
        ts: new Date().toISOString(),
        service: 'brevio-metrics',
        event: 'metrics.start.failed',
        severity: 'ERROR',
        message: err instanceof Error ? err.message : String(err)
      }) + '\n'
    );
    process.exit(1);
  });
}

export { buildRuntime as createMetricsRuntime };
