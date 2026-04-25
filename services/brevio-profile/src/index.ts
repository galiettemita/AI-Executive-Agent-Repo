import { randomUUID } from 'node:crypto';
import { existsSync } from 'node:fs';
import { mkdir } from 'node:fs/promises';
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
import { KNOWLEDGE_FILES, ProfileStore, type KnowledgeFileName } from './profile-store.js';

interface ProfileConfig {
  serviceName: string;
  version: string;
  environment: string;
  port: number;
  shutdownTimeoutMs: number;
  profilesRootDir: string;
  maxKnowledgeBytes: number;
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

interface ProfileRuntime {
  config: ProfileConfig;
  startedAtMs: number;
  store: ProfileStore;
  server: http.Server;
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

function loadConfig(): ProfileConfig {
  const environment = loadBrevioEnvironment();
  return {
    serviceName: 'brevio-profile',
    version: process.env.SERVICE_VERSION ?? '0.2.0',
    environment,
    port: parsePositiveInt(process.env.PORT, 8084, 'PORT'),
    shutdownTimeoutMs: parsePositiveInt(process.env.BREVIO_PROFILE_SHUTDOWN_TIMEOUT_MS, 30000, 'BREVIO_PROFILE_SHUTDOWN_TIMEOUT_MS'),
    profilesRootDir: path.resolve(process.env.BREVIO_PROFILE_DATA_DIR ?? path.join(process.cwd(), 'data', 'profiles')),
    maxKnowledgeBytes: parsePositiveInt(process.env.BREVIO_PROFILE_MAX_KNOWLEDGE_BYTES, 512 * 1024, 'BREVIO_PROFILE_MAX_KNOWLEDGE_BYTES'),
    internalAuthSecret: resolveAccessTokenVerificationKey(
      process.env.BREVIO_INTERNAL_AUTH_PUBLIC_KEY,
      process.env.BREVIO_INTERNAL_AUTH_PRIVATE_KEY,
      process.env.BREVIO_INTERNAL_AUTH_SECRET,
      environment,
      'BREVIO_INTERNAL_AUTH_PUBLIC_KEY',
      'brevio-profile'
    ),
    internalAuthIssuer: process.env.BREVIO_INTERNAL_AUTH_ISSUER?.trim() || 'https://auth.brevio.internal',
    serviceAudience: process.env.BREVIO_PROFILE_AUDIENCE?.trim() || 'brevio-profile',
    callerContextSecret: requireSharedSecret(process.env.BREVIO_CALLER_CONTEXT_SECRET, 'BREVIO_CALLER_CONTEXT_SECRET', environment, 'brevio-profile-caller'),
    logSalt: process.env.BREVIO_PROFILE_LOG_SALT?.trim() || `brevio-profile:${environment}`
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
  runtime: ProfileRuntime,
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

function resolveKnowledgeFile(raw: string | undefined): KnowledgeFileName | undefined {
  const normalized = (raw ?? '').trim().toLowerCase();
  if (normalized === 'user' || normalized === 'user.md') {
    return 'USER.md';
  }
  if (normalized === 'soul' || normalized === 'soul.md') {
    return 'SOUL.md';
  }
  if (normalized === 'agents' || normalized === 'agents.md') {
    return 'AGENTS.md';
  }
  return undefined;
}

function parseApiPath(pathname: string): { base: 'api' | 'v1'; segments: string[] } | undefined {
  const segments = pathname.split('/').filter((segment) => segment.length > 0);
  if (segments.length < 2) {
    return undefined;
  }

  if (segments[0] === 'api' && segments[1] === 'v1') {
    return { base: 'api', segments: segments.slice(2) };
  }

  if (segments[0] === 'v1') {
    return { base: 'v1', segments: segments.slice(1) };
  }

  return undefined;
}

function healthPayload(runtime: ProfileRuntime, deep: boolean): Record<string, unknown> {
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
      profile_storage_mode: runtime.store.mode(),
      profile_storage_durable: true,
      shared_storage_backend: false,
      profile_storage_exists: existsSync(runtime.config.profilesRootDir)
    },
    profile_storage: {
      root_dir: runtime.config.profilesRootDir,
      max_knowledge_bytes: runtime.config.maxKnowledgeBytes
    }
  };
}

function buildRuntime(config?: ProfileConfig): ProfileRuntime {
  const resolvedConfig = config ?? loadConfig();
  const startedAtMs = Date.now();
  const store = new ProfileStore(resolvedConfig.profilesRootDir);

  let runtimeRef: ProfileRuntime | undefined;

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
      logEvent(runtime, ctx, 'profile.request.error', 'WARN', {
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

    const parsedPath = parseApiPath(pathname);
    if (!parsedPath || parsedPath.segments[0] !== 'profile') {
      onError(404, 'not_found');
      return;
    }

    const segments = parsedPath.segments;
    const userId = sanitizeUserId(segments[1]);
    if (!userId) {
      onError(400, 'invalid_user_id');
      return;
    }
    let auth: ReturnType<typeof authenticateInternalRequest>;
    try {
      auth = authenticateInternalRequest(req, runtime.config, ctx);
    } catch (error) {
      onError(401, error instanceof Error ? error.message : 'authorization_required');
      return;
    }
    let effectiveScope;
    try {
      effectiveScope = resolveEffectiveUserScope(auth, { requireUserId: auth.principal.token_use !== 'admin_access' });
    } catch (error) {
      onError(400, error instanceof Error ? error.message : 'caller_context_required');
      return;
    }
    if (auth.principal.token_use !== 'admin_access' && effectiveScope.userId !== userId) {
      onError(403, 'forbidden');
      return;
    }

    if (method === 'GET' && segments.length === 2) {
      void (async () => {
        const profile = await runtime.store.ensureProfile(userId);
        sendJSON(res, 200, {
          user_id: profile.user_id,
          timezone: profile.timezone,
          locale: profile.locale,
          preferences: profile.preferences,
          profile_hash: profile.profile_hash,
          knowledge_files: KNOWLEDGE_FILES,
          created_at: profile.created_at,
          updated_at: profile.updated_at
        });
        logEvent(runtime, ctx, 'profile.fetch.complete', 'INFO', {
          user_id: userId
        });
      })().catch((err) => {
        onError(500, 'profile_fetch_failed');
        logEvent(runtime, ctx, 'profile.fetch.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'PUT' && segments.length === 3 && segments[2] === 'preferences') {
      void (async () => {
        const rawBody = await readRawBody(req, runtime.config.maxKnowledgeBytes);
        const payload = parseObject(rawBody);
        const preferences = asObject(payload.preferences);
        if (!preferences) {
          onError(400, 'preferences_required');
          return;
        }

        const profile = await runtime.store.updatePreferences(userId, {
          preferences,
          timezone: asString(payload.timezone),
          locale: asString(payload.locale)
        });

        sendJSON(res, 200, {
          user_id: profile.user_id,
          timezone: profile.timezone,
          locale: profile.locale,
          preferences: profile.preferences,
          profile_hash: profile.profile_hash,
          updated_at: profile.updated_at
        });

        logEvent(runtime, ctx, 'profile.preferences.updated', 'INFO', {
          user_id: userId
        });
      })().catch((err) => {
        if (err instanceof Error && err.message === 'payload_too_large') {
          onError(413, 'payload_too_large');
          return;
        }
        if (err instanceof Error && err.message === 'invalid_json') {
          onError(400, 'invalid_json');
          return;
        }
        onError(500, 'profile_update_failed');
        logEvent(runtime, ctx, 'profile.preferences.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'GET' && segments.length === 4 && segments[2] === 'knowledge') {
      const knowledgeFile = resolveKnowledgeFile(segments[3]);
      if (!knowledgeFile) {
        onError(400, 'invalid_knowledge_file');
        return;
      }

      void (async () => {
        const { profile, content } = await runtime.store.readKnowledge(userId, knowledgeFile);

        sendJSON(res, 200, {
          user_id: userId,
          file: knowledgeFile,
          content,
          profile_hash: profile.profile_hash,
          updated_at: profile.updated_at
        });

        logEvent(runtime, ctx, 'profile.knowledge.fetch.complete', 'INFO', {
          user_id: userId,
          file: knowledgeFile,
          bytes: Buffer.byteLength(content, 'utf8')
        });
      })().catch((err) => {
        onError(500, 'knowledge_fetch_failed');
        logEvent(runtime, ctx, 'profile.knowledge.fetch.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'PUT' && segments.length === 4 && segments[2] === 'knowledge') {
      const knowledgeFile = resolveKnowledgeFile(segments[3]);
      if (!knowledgeFile) {
        onError(400, 'invalid_knowledge_file');
        return;
      }

      void (async () => {
        const rawBody = await readRawBody(req, runtime.config.maxKnowledgeBytes);
        const payload = parseObject(rawBody);
        const content = asString(payload.content);
        if (typeof content !== 'string') {
          onError(400, 'content_required');
          return;
        }

        const profile = await runtime.store.writeKnowledge(userId, knowledgeFile, content);

        sendJSON(res, 200, {
          user_id: userId,
          file: knowledgeFile,
          profile_hash: profile.profile_hash,
          bytes_written: Buffer.byteLength(content, 'utf8'),
          updated_at: profile.updated_at
        });

        logEvent(runtime, ctx, 'profile.knowledge.updated', 'INFO', {
          user_id: userId,
          file: knowledgeFile,
          bytes_written: Buffer.byteLength(content, 'utf8')
        });
      })().catch((err) => {
        if (err instanceof Error && err.message === 'payload_too_large') {
          onError(413, 'payload_too_large');
          return;
        }
        if (err instanceof Error && err.message === 'invalid_json') {
          onError(400, 'invalid_json');
          return;
        }
        onError(500, 'knowledge_update_failed');
        logEvent(runtime, ctx, 'profile.knowledge.update.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    if (method === 'POST' && segments.length === 4 && segments[2] === 'hash' && segments[3] === 'refresh') {
      void (async () => {
        const profile = await runtime.store.refreshHash(userId);

        sendJSON(res, 200, {
          user_id: userId,
          profile_hash: profile.profile_hash,
          updated_at: profile.updated_at
        });

        logEvent(runtime, ctx, 'profile.hash.refresh.complete', 'INFO', {
          user_id: userId,
          profile_hash: profile.profile_hash
        });
      })().catch((err) => {
        onError(500, 'hash_refresh_failed');
        logEvent(runtime, ctx, 'profile.hash.refresh.exception', 'ERROR', {
          message: err instanceof Error ? err.message : String(err)
        });
      });
      return;
    }

    onError(404, 'not_found');
  });

  const runtime: ProfileRuntime = {
    config: resolvedConfig,
    startedAtMs,
    store,
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

function installSignalHandlers(runtime: ProfileRuntime): void {
  const shutdown = async (signal: string): Promise<void> => {
    const ctx: RequestContext = {
      traceId: randomUUID(),
      spanId: randomUUID(),
      requestId: randomUUID()
    };

    logEvent(runtime, ctx, 'profile.shutdown.start', 'INFO', { signal });

    const timeout = setTimeout(() => {
      logEvent(runtime, ctx, 'profile.shutdown.timeout', 'ERROR', {
        timeout_ms: runtime.config.shutdownTimeoutMs
      });
      process.exit(1);
    }, runtime.config.shutdownTimeoutMs);

    try {
      await runtime.close();
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'profile.shutdown.complete', 'INFO', {});
      process.exit(0);
    } catch (err) {
      clearTimeout(timeout);
      logEvent(runtime, ctx, 'profile.shutdown.failed', 'ERROR', {
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

  await mkdir(runtime.config.profilesRootDir, { recursive: true });

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

  logEvent(runtime, ctx, 'profile.started', 'INFO', {
    port: runtime.config.port,
    profiles_root_dir: runtime.config.profilesRootDir,
    max_knowledge_bytes: runtime.config.maxKnowledgeBytes
  });
}

if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url) {
  void main().catch((err) => {
    process.stderr.write(
      JSON.stringify({
        ts: new Date().toISOString(),
        service: 'brevio-profile',
        event: 'profile.start.failed',
        severity: 'ERROR',
        message: err instanceof Error ? err.message : String(err)
      }) + '\n'
    );
    process.exit(1);
  });
}

export { buildRuntime as createProfileRuntime };
