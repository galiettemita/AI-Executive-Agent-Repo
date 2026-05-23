import { randomUUID } from 'node:crypto';
import http from 'node:http';
import { pathToFileURL } from 'node:url';

import { loadFomoConfig } from './config.js';
import { handleHealth } from './routes/health.js';
import { tryHandleOAuthGoogleRequest, type OAuthGoogleRouteDeps } from './routes/oauth-google.js';
import { GmailClient } from './adapters/gmail/client.js';
import { createStores, type SubstrateStoresHandle } from './db/store-factory.js';
import { loadCryptoConfig } from './security/token-crypto.js';
import { loadSessionConfig } from './security/session.js';
import { InMemoryNonceStore, loadOAuthStateConfig } from './security/oauth/state.js';
import { loadProviderConfig } from './security/oauth/providers/index.js';
import type { FomoConfig, RequestContext } from './types.js';

interface FomoRuntime {
  config: FomoConfig;
  server: http.Server;
  startedAtMs: number;
  storesHandle: SubstrateStoresHandle;
  close(): Promise<void>;
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
  config: FomoConfig,
  ctx: RequestContext | undefined,
  event: string,
  severity: 'INFO' | 'WARN' | 'ERROR',
  attrs: Record<string, unknown>
): void {
  process.stdout.write(
    JSON.stringify({
      ts: new Date().toISOString(),
      service: config.serviceName,
      env: config.environment,
      trace_id: ctx?.traceId,
      span_id: ctx?.spanId,
      request_id: ctx?.requestId,
      user_id: ctx?.userId,
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

// Build OAuth route deps from env. Returns null if Google OAuth is not
// configured (GOOGLE_CLIENT_ID / SECRET / REDIRECT_URI env vars missing).
// In that case the server still boots and /health works; OAuth routes
// just don't exist. Production deploys should have all three set.
function buildOAuthGoogleDeps(
  storesHandle: SubstrateStoresHandle,
  config: FomoConfig
): OAuthGoogleRouteDeps | null {
  const providerConfig = loadProviderConfig('google');
  if (!providerConfig) {
    logEvent(config, undefined, 'fomo.oauth.google.not_configured', 'WARN', {
      detail: 'GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / BREVIO_OAUTH_REDIRECT_URI_GOOGLE not set — /oauth/google/* routes disabled'
    });
    return null;
  }
  return {
    providerConfig,
    stateConfig: loadOAuthStateConfig(),
    nonceStore: new InMemoryNonceStore(),
    tokenStore: storesHandle.stores.tokens,
    gmailCursorStore: storesHandle.stores.gmailCursors,
    gmailClient: new GmailClient(),
    sessionConfig: loadSessionConfig()
  };
}

export function createFomoRuntime(config: FomoConfig = loadFomoConfig()): FomoRuntime {
  const startedAtMs = Date.now();

  // Substrate stores — throws in production if BREVIO_TOKEN_KEK missing
  // and BREVIO_DEV_MODE is not 'true'. Same fail-closed behavior as the
  // Phase 2E client.
  const cryptoConfig = loadCryptoConfig();
  const storesHandle = createStores({ env: process.env, crypto: cryptoConfig });

  // OAuth routes — graceful skip when not configured.
  const oauthGoogleDeps = buildOAuthGoogleDeps(storesHandle, config);

  const server = http.createServer((req, res) => {
    const ctx = requestContext(req);
    const method = req.method ?? 'GET';
    const path = (req.url ?? '/').split('?')[0] ?? '/';

    if (method === 'GET' && path === '/health') {
      handleHealth(res, config, startedAtMs);
      return;
    }

    // OAuth routes (only when wired).
    if (oauthGoogleDeps) {
      void tryHandleOAuthGoogleRequest(req, oauthGoogleDeps)
        .then((response) => {
          if (response) {
            res.writeHead(response.status, response.headers);
            res.end(response.body);
            return;
          }
          sendJSON(res, 404, { error: 'not_found', request_id: ctx.requestId });
        })
        .catch((err: unknown) => {
          logEvent(config, ctx, 'fomo.oauth.google.unhandled', 'ERROR', {
            error: err instanceof Error ? err.message : String(err)
          });
          if (!res.headersSent) {
            sendJSON(res, 500, { error: 'internal', request_id: ctx.requestId });
          }
        });
      return;
    }

    sendJSON(res, 404, { error: 'not_found', request_id: ctx.requestId });
  });

  const runtime: FomoRuntime = {
    config,
    server,
    startedAtMs,
    storesHandle,
    close: async () => {
      await new Promise<void>((resolve, reject) => {
        server.close((err) => (err ? reject(err) : resolve()));
      });
      if (storesHandle.db?.ok) {
        await storesHandle.db.pool.end();
      }
    }
  };

  server.on('listening', () => {
    logEvent(config, undefined, 'fomo.server.listening', 'INFO', {
      port: config.port,
      store_backend: storesHandle.backend,
      oauth_google_wired: oauthGoogleDeps !== null
    });
  });

  return runtime;
}

export async function main(): Promise<void> {
  const runtime = createFomoRuntime();
  runtime.server.listen(runtime.config.port);

  const shutdown = async (signal: string): Promise<void> => {
    logEvent(runtime.config, undefined, 'fomo.server.shutting_down', 'INFO', { signal });
    try {
      await runtime.close();
    } catch (err) {
      logEvent(runtime.config, undefined, 'fomo.server.shutdown_error', 'ERROR', {
        error: err instanceof Error ? err.message : String(err)
      });
      process.exitCode = 1;
    }
  };

  process.once('SIGTERM', () => {
    void shutdown('SIGTERM');
  });
  process.once('SIGINT', () => {
    void shutdown('SIGINT');
  });
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  void main();
}
