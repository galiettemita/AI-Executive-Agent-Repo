import { randomUUID } from 'node:crypto';
import { existsSync } from 'node:fs';
import http from 'node:http';
import { pathToFileURL } from 'node:url';

import {
  extractBearerToken,
  hashTokenBinding,
  pseudonymizedRef,
  signAccessToken,
  signCallerContextEnvelope,
  verifyAccessToken
} from '../../../packages/shared/src/security.js';
import { loadGatewayConfig } from './config.js';
import { formatOutboundText } from './format.js';
import { normalizeWebhook } from './normalize.js';
import { verifyAPIKey, verifyWhatsAppSignature } from './security.js';
import { GatewayState } from './state.js';
import type { Channel, GatewayConfig, RequestContext, UserTier } from './types.js';
import { startMessageWorkflow } from './workflow-runtime.js';

interface GatewayRuntime {
  config: GatewayConfig;
  state: GatewayState;
  startedAtMs: number;
  server: http.Server;
  close(): Promise<void>;
}

const WHATSAPP_PATHS = ['/webhooks/whatsapp', '/api/v1/webhooks/whatsapp', '/v1/gateway/webhook/whatsapp'];
const IMESSAGE_PATHS = ['/webhooks/imessage', '/api/v1/webhooks/imessage', '/v1/gateway/webhook/imessage'];
const TEMPORAL_PATHS = ['/webhooks/temporal', '/api/v1/webhooks/temporal'];

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
  config: GatewayConfig,
  ctx: RequestContext,
  event: string,
  severity: 'INFO' | 'WARN' | 'ERROR',
  attrs: Record<string, unknown>
): void {
  process.stdout.write(
    JSON.stringify({
      ts: new Date().toISOString(),
      service: config.serviceName,
      env: config.environment,
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

function requireAdminRequest(req: http.IncomingMessage, config: GatewayConfig, ctx: RequestContext): void {
  const token = extractBearerToken(getHeader(req, 'authorization'));
  if (!token) {
    throw new Error('admin_token_required');
  }
  const principal = verifyAccessToken(config.accessTokenIssuers, token, {
    expectedAudience: config.serviceAudience,
    expectedIssuer: config.adminTokenIssuer,
    allowedTokenUses: ['admin_access']
  });
  ctx.subjectRef = pseudonymizedRef(principal.sub, config.logSalt);
}

function parseTier(rawTier: string | undefined): UserTier {
  const normalized = rawTier?.trim().toLowerCase();
  switch (normalized) {
    case 'free':
    case 'pro':
    case 'enterprise':
    case 'admin':
    case 'service':
      return normalized;
    default:
      return 'free';
  }
}

function sendJSON(res: http.ServerResponse, statusCode: number, payload: Record<string, unknown>): void {
  res.writeHead(statusCode, { 'content-type': 'application/json' });
  res.end(JSON.stringify(payload));
}

function pathMatches(pathname: string, candidates: string[]): boolean {
  return candidates.includes(pathname);
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

function parseJSON(rawBody: Buffer): unknown {
  try {
    return JSON.parse(rawBody.toString('utf8'));
  } catch {
    return {};
  }
}

function healthPayload(runtime: GatewayRuntime, deep: boolean): Record<string, unknown> {
  const base: Record<string, unknown> = {
    status: 'healthy',
    version: runtime.config.version,
    uptime_ms: Date.now() - runtime.startedAtMs
  };

  if (!deep) {
    return base;
  }

  const stats = runtime.state.stats();

  return {
    ...base,
    checks: {
      process: 'ok',
      webhook_secrets: runtime.config.whatsappWebhookSecret.trim() !== '' ? 'configured' : 'not_configured',
      imessage_api_key: runtime.config.imessageAPIKey.trim() !== '' ? 'configured' : 'not_configured',
      temporal_api_key: runtime.config.temporalWebhookAPIKey.trim() !== '' ? 'configured' : 'not_configured',
      state_backend: runtime.state.mode(),
      durable_state: runtime.state.mode() !== 'in_memory',
      shared_state_backend: false,
      state_file_exists: runtime.state.snapshotPath() ? existsSync(runtime.state.snapshotPath()!) : false
    },
    idempotency: {
      ttl_ms: runtime.config.idempotencyTtlMs,
      cache_entries: stats.dedupEntries
    },
    sessions: {
      idle_rotation_ms: runtime.config.sessionIdleMs,
      tracked: stats.activeSessions
    },
    rate_limit: {
      minute_limit: runtime.config.rateLimitPerMinute,
      free_hour_limit: runtime.config.rateLimitFreePerHour,
      pro_hour_limit: runtime.config.rateLimitProPerHour,
      tracked_users: stats.trackedUsers
    }
  };
}

function shouldAllowWhatsAppRequest(runtime: GatewayRuntime, req: http.IncomingMessage, rawBody: Buffer): boolean {
  const signature = getHeader(req, 'x-hub-signature-256');
  if (runtime.config.whatsappWebhookSecret.trim() === '') {
    return runtime.config.environment === 'local' || runtime.config.environment === 'test';
  }
  return verifyWhatsAppSignature(rawBody, signature, runtime.config.whatsappWebhookSecret);
}

function shouldAllowIMessageRequest(runtime: GatewayRuntime, req: http.IncomingMessage): boolean {
  const provided = getHeader(req, 'x-api-key');
  return verifyAPIKey(provided, runtime.config.imessageAPIKey, runtime.config.environment);
}

function shouldAllowTemporalRequest(runtime: GatewayRuntime, req: http.IncomingMessage): boolean {
  const provided = getHeader(req, 'x-api-key');
  return verifyAPIKey(provided, runtime.config.temporalWebhookAPIKey, runtime.config.environment);
}

function verifyWhatsAppChallenge(runtime: GatewayRuntime, req: http.IncomingMessage, res: http.ServerResponse): boolean {
  const url = new URL(req.url ?? '/', 'http://localhost');
  const mode = url.searchParams.get('hub.mode');
  const verifyToken = url.searchParams.get('hub.verify_token');
  const challenge = url.searchParams.get('hub.challenge') ?? '';

  if (mode !== 'subscribe') {
    sendJSON(res, 400, { error: 'invalid_challenge_mode' });
    return false;
  }

  if (runtime.config.whatsappVerifyToken.trim() === '') {
    if (runtime.config.environment !== 'local' && runtime.config.environment !== 'test') {
      sendJSON(res, 500, { error: 'verify_token_not_configured' });
      return false;
    }
    res.writeHead(200, { 'content-type': 'text/plain' });
    res.end(challenge);
    return true;
  }

  if (verifyToken !== runtime.config.whatsappVerifyToken) {
    sendJSON(res, 403, { error: 'verify_token_mismatch' });
    return false;
  }

  res.writeHead(200, { 'content-type': 'text/plain' });
  res.end(challenge);
  return true;
}

async function handleWebhook(
  runtime: GatewayRuntime,
  req: http.IncomingMessage,
  res: http.ServerResponse,
  channel: Channel,
  authCheck: (rawBody: Buffer) => boolean,
  ctx: RequestContext
): Promise<void> {
  const nowMs = Date.now();
  runtime.state.prune(nowMs);

  const rawBody = await readRawBody(req);

  if (!authCheck(rawBody)) {
    logEvent(runtime.config, ctx, 'gateway.webhook.unauthorized', 'WARN', { channel });
    sendJSON(res, 401, { error: 'unauthorized', channel });
    return;
  }

  const payload = parseJSON(rawBody);
  const tier = parseTier(getHeader(req, 'x-user-tier'));

  const normalized = normalizeWebhook(channel, payload, rawBody, nowMs, runtime.state, runtime.config, tier);
  const cached = runtime.state.getCachedResponse(normalized.dedupKey, nowMs);
  if (cached) {
    sendJSON(res, cached.statusCode, {
      ...cached.payload,
      idempotent_replay: true
    });
    logEvent(runtime.config, ctx, 'gateway.webhook.idempotent_replay', 'INFO', {
      channel,
      dedup_key: normalized.dedupKey,
      user_id: normalized.userId
    });
    return;
  }

  const rateDecision = runtime.state.checkRateLimit(normalized.userId, normalized.tier, nowMs, runtime.config);
  if (!rateDecision.allowed) {
    sendJSON(res, 429, {
      error: 'rate_limited',
      reason: rateDecision.reason,
      retry_after_seconds: rateDecision.retryAfterSeconds,
      limit: rateDecision.limit
    });
    logEvent(runtime.config, ctx, 'gateway.webhook.rate_limited', 'WARN', {
      channel,
      user_id: normalized.userId,
      reason: rateDecision.reason,
      retry_after_seconds: rateDecision.retryAfterSeconds
    });
    return;
  }

  const serviceToken = signAccessToken(runtime.config.serviceTokenSigningKey, {
    version: 2,
    sub: runtime.config.serviceName,
    iss: runtime.config.serviceTokenIssuer,
    aud: runtime.config.temporalWorkerAudience,
    iat: Math.floor(nowMs / 1000),
    exp: Math.floor((nowMs + 2 * 60 * 1000) / 1000),
    token_use: 'service_access',
    scopes: ['workflow:start'],
    service: runtime.config.serviceName
  });
  const callerContextToken = signCallerContextEnvelope(runtime.config.callerContextSigningKey, {
        iss: runtime.config.callerContextIssuer,
        aud: runtime.config.temporalWorkerAudience,
        sub: normalized.userId,
        user_id: normalized.userId,
        workspace_id: normalized.envelope.context.workspace_id,
        tenant_id: normalized.envelope.context.tenant_id,
        actor_service: runtime.config.serviceName,
        channel,
        channel_subject: normalized.channelSubject,
        auth_strength: channel === 'WHATSAPP' ? 'webhook_signature' : 'api_key',
        provenance: `gateway:${channel.toLowerCase()}`,
        jti: `${normalized.envelope.id}:${ctx.requestId}`,
        iat: Math.floor(nowMs / 1000),
        nbf: Math.floor(nowMs / 1000),
        exp: Math.floor((nowMs + 5 * 60 * 1000) / 1000),
        ath: hashTokenBinding(serviceToken)
      });
  const runtimeStart = await startMessageWorkflow(
    {
      messageId: normalized.envelope.id,
      channel,
      channelMessageId: normalized.envelope.metadata.channel_message_id,
      sessionId: normalized.envelope.metadata.session_id,
      messageText: normalized.envelope.content.text,
      userProfileHash: normalized.envelope.context.user_profile_hash,
      serviceToken,
      callerContextToken
    },
    runtime.config
  );

  if (runtimeStart.warning) {
    logEvent(runtime.config, ctx, 'gateway.workflow_runtime.start_skipped', 'WARN', {
      channel,
      user_id: normalized.userId,
      warning: runtimeStart.warning
    });
  }

  const accepted: Record<string, unknown> = {
    status: 'accepted',
    channel,
    message_id: normalized.envelope.id,
    run_id: runtimeStart.runId ?? normalized.envelope.id,
    thread_id: normalized.envelope.metadata.session_id,
    channel_message_id: normalized.envelope.metadata.channel_message_id,
    session_id: normalized.envelope.metadata.session_id,
    envelope: normalized.envelope,
    next_stage: runtimeStart.delegated ? 'temporal-worker.message-processing' : 'brain.classify',
    workflow_runtime: runtimeStart.delegated ? 'temporal-worker' : 'local',
    idempotent_replay: false
  };

  runtime.state.cacheResponse(normalized.dedupKey, 202, accepted, nowMs, runtime.config.idempotencyTtlMs);

  sendJSON(res, 202, accepted);
  logEvent(runtime.config, ctx, 'gateway.webhook.accepted', 'INFO', {
    channel,
    user_id: normalized.userId,
    tier: normalized.tier,
    dedup_key: normalized.dedupKey,
    remaining_hour_budget: rateDecision.remaining
  });
}

async function handleTemporalWebhook(
  runtime: GatewayRuntime,
  req: http.IncomingMessage,
  res: http.ServerResponse,
  ctx: RequestContext
): Promise<void> {
  const rawBody = await readRawBody(req);
  if (!shouldAllowTemporalRequest(runtime, req)) {
    logEvent(runtime.config, ctx, 'gateway.temporal.unauthorized', 'WARN', {});
    sendJSON(res, 401, { error: 'unauthorized' });
    return;
  }

  const payload = parseJSON(rawBody);
  const payloadObject = payload && typeof payload === 'object' && !Array.isArray(payload)
    ? (payload as Record<string, unknown>)
    : {};

  const workflowRunID = typeof payloadObject.workflow_run_id === 'string'
    ? payloadObject.workflow_run_id
    : undefined;
  const dedupKey = workflowRunID ? `TEMPORAL:${workflowRunID}` : undefined;

  if (dedupKey) {
    const cached = runtime.state.getCachedResponse(dedupKey, Date.now());
    if (cached) {
      sendJSON(res, cached.statusCode, {
        ...cached.payload,
        idempotent_replay: true
      });
      return;
    }
  }

  const response: Record<string, unknown> = {
    status: 'acknowledged',
    workflow_run_id: workflowRunID,
    idempotent_replay: false
  };

  if (dedupKey) {
    runtime.state.cacheResponse(dedupKey, 202, response, Date.now(), runtime.config.idempotencyTtlMs);
  }

  sendJSON(res, 202, response);
}

async function handleFormat(runtime: GatewayRuntime, req: http.IncomingMessage, res: http.ServerResponse): Promise<void> {
  const rawBody = await readRawBody(req);
  const payloadRaw = parseJSON(rawBody);
  const payload = payloadRaw && typeof payloadRaw === 'object' && !Array.isArray(payloadRaw)
    ? (payloadRaw as Record<string, unknown>)
    : {};

  const channelRaw = typeof payload.channel === 'string' ? payload.channel.toUpperCase() : 'API';
  const channel: Channel = channelRaw === 'WHATSAPP' || channelRaw === 'IMESSAGE' ? channelRaw : 'API';
  const text = typeof payload.text === 'string' ? payload.text : '';

  if (text.trim() === '') {
    sendJSON(res, 400, { error: 'text_required' });
    return;
  }

  const formatted = formatOutboundText(channel, text);

  sendJSON(res, 200, {
    channel,
    formatted_text: formatted,
    chars: formatted.length
  });
}

function createGatewayRuntime(config?: GatewayConfig): GatewayRuntime {
  const resolvedConfig = config ?? loadGatewayConfig();
  const state = new GatewayState(resolvedConfig.stateFilePath);
  const startedAtMs = Date.now();
  let runtimeRef: GatewayRuntime | undefined;

  const server = http.createServer((req, res) => {
    const ctx = requestContext(req);
    const method = req.method ?? 'GET';
    const url = new URL(req.url ?? '/', 'http://localhost');
    const pathname = url.pathname;

    const onError = (statusCode: number, code: string): void => {
      sendJSON(res, statusCode, { error: code });
      logEvent(resolvedConfig, ctx, 'gateway.request.error', 'WARN', {
        method,
        path: pathname,
        status_code: statusCode,
        code
      });
    };

    if (method === 'GET' && pathname === '/health') {
      const runtime = runtimeRef;
      if (!runtime) {
        onError(500, 'runtime_not_ready');
        return;
      }
      sendJSON(res, 200, healthPayload(runtime, false));
      return;
    }

    if (method === 'GET' && pathname === '/health/deep') {
      const runtime = runtimeRef;
      if (!runtime) {
        onError(500, 'runtime_not_ready');
        return;
      }
      try {
        requireAdminRequest(req, resolvedConfig, ctx);
      } catch (error) {
        onError(401, error instanceof Error ? error.message : 'unauthorized');
        return;
      }
      sendJSON(res, 200, healthPayload(runtime, true));
      return;
    }

    if (method === 'GET' && pathMatches(pathname, WHATSAPP_PATHS)) {
      const runtime = runtimeRef;
      if (!runtime) {
        onError(500, 'runtime_not_ready');
        return;
      }
      const ok = verifyWhatsAppChallenge(runtime, req, res);
      if (ok) {
        logEvent(resolvedConfig, ctx, 'gateway.whatsapp.challenge', 'INFO', { path: pathname });
      }
      return;
    }

    if (method === 'POST' && pathMatches(pathname, WHATSAPP_PATHS)) {
      const runtime = runtimeRef;
      if (!runtime) {
        onError(500, 'runtime_not_ready');
        return;
      }
      void handleWebhook(runtime, req, res, 'WHATSAPP', (rawBody) => shouldAllowWhatsAppRequest(runtime, req, rawBody), ctx).catch((err) => {
        onError(500, 'webhook_processing_failed');
        logEvent(resolvedConfig, ctx, 'gateway.webhook.exception', 'ERROR', {
          channel: 'WHATSAPP',
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'POST' && pathMatches(pathname, IMESSAGE_PATHS)) {
      const runtime = runtimeRef;
      if (!runtime) {
        onError(500, 'runtime_not_ready');
        return;
      }
      void handleWebhook(runtime, req, res, 'IMESSAGE', () => shouldAllowIMessageRequest(runtime, req), ctx).catch((err) => {
        onError(500, 'webhook_processing_failed');
        logEvent(resolvedConfig, ctx, 'gateway.webhook.exception', 'ERROR', {
          channel: 'IMESSAGE',
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'POST' && pathMatches(pathname, TEMPORAL_PATHS)) {
      const runtime = runtimeRef;
      if (!runtime) {
        onError(500, 'runtime_not_ready');
        return;
      }
      void handleTemporalWebhook(runtime, req, res, ctx).catch((err) => {
        onError(500, 'temporal_webhook_failed');
        logEvent(resolvedConfig, ctx, 'gateway.temporal.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/gateway/format') {
      const runtime = runtimeRef;
      if (!runtime) {
        onError(500, 'runtime_not_ready');
        return;
      }
      void handleFormat(runtime, req, res).catch((err) => {
        onError(500, 'format_failed');
        logEvent(resolvedConfig, ctx, 'gateway.format.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    onError(404, 'not_found');
  });

  const runtime: GatewayRuntime = {
    config: resolvedConfig,
    state,
    startedAtMs,
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

function installSignalHandlers(runtime: GatewayRuntime): void {
  const shutdown = async (signal: string): Promise<void> => {
    const ctx: RequestContext = {
      traceId: randomUUID(),
      spanId: randomUUID(),
      requestId: randomUUID()
    };

    logEvent(runtime.config, ctx, 'gateway.shutdown.start', 'INFO', { signal });

    const timeout = setTimeout(() => {
      logEvent(runtime.config, ctx, 'gateway.shutdown.timeout', 'ERROR', {
        timeout_ms: runtime.config.shutdownTimeoutMs
      });
      process.exit(1);
    }, runtime.config.shutdownTimeoutMs);

    try {
      await runtime.close();
      clearTimeout(timeout);
      logEvent(runtime.config, ctx, 'gateway.shutdown.complete', 'INFO', {});
      process.exit(0);
    } catch (err) {
      clearTimeout(timeout);
      logEvent(runtime.config, ctx, 'gateway.shutdown.failed', 'ERROR', {
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
  const runtime = createGatewayRuntime();

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

  logEvent(runtime.config, ctx, 'gateway.started', 'INFO', {
    port: runtime.config.port,
    whatsapp_paths: WHATSAPP_PATHS,
    imessage_paths: IMESSAGE_PATHS,
    temporal_paths: TEMPORAL_PATHS,
    rate_limits: {
      minute: runtime.config.rateLimitPerMinute,
      free_per_hour: runtime.config.rateLimitFreePerHour,
      pro_per_hour: runtime.config.rateLimitProPerHour
    },
    idempotency_ttl_ms: runtime.config.idempotencyTtlMs
  });
}

if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url) {
  void main().catch((err) => {
    process.stderr.write(
      JSON.stringify({
        ts: new Date().toISOString(),
        service: 'brevio-gateway',
        event: 'gateway.start.failed',
        severity: 'ERROR',
        error: err instanceof Error ? err.message : String(err)
      }) + '\n'
    );
    process.exit(1);
  });
}

export { createGatewayRuntime };
