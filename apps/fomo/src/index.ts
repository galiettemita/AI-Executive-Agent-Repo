import { randomUUID } from 'node:crypto';
import http from 'node:http';
import { pathToFileURL } from 'node:url';

import { loadFomoConfig } from './config.js';
import { handleHealth } from './routes/health.js';
import { tryHandleOAuthGoogleRequest, type OAuthGoogleRouteDeps } from './routes/oauth-google.js';
import { GmailClient } from './adapters/gmail/client.js';
import { createStores, type SubstrateStoresHandle } from './db/store-factory.js';
import { loadKillSwitches, type KillSwitches } from './core/kill-switches.js';
import { createToolRegistry } from './core/tool-registry.js';
import { type PolicyGateDeps } from './core/policy-gate.js';
import { createDispatchTable, type DispatchTable } from './dispatch/dispatcher.js';
import { wireInternalExecutors } from './dispatch/internal-executors.js';
import { wireExternalExecutors } from './dispatch/external-executors.js';
import { runOnce as runPollOnce, type GmailPollRankerDep } from './workers/gmail-poll.js';
import { OpenAIBackend } from './core/model-backends/openai.js';
import { createModelRouter } from './core/model-router.js';
import { rankEmail } from './ranker/index.js';
import { RANKER_OPENAI_RESPONSE_FORMAT } from './ranker/openai-response-format.js';
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

interface PollingHandle {
  stop(): Promise<void>;
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
  config: FomoConfig,
  gmailClient: GmailClient
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
    gmailClient,
    sessionConfig: loadSessionConfig()
  };
}

/* ---------------------------------------------------------------------- */
/* Polling bootstrap (Phase 3B.2)                                         */
/* ---------------------------------------------------------------------- */

// hasConsent / hasOAuth are sync callbacks per the Permission Gate API.
// The polling worker iterates users per cycle, but the gate evaluations
// happen synchronously inside runOnce. We refresh a token-state snapshot
// once at the top of each cycle and the callbacks read from it.
//
// Identity: for v0.1, completing OAuth IS consent for gmail.read — the
// founder explicitly granted by walking through /oauth/google/start.
// Future phases that introduce a separate consent surface (e.g., per-tool
// consent toggles, friend-beta gating) will plug a real ConsentStore in
// here without changing the worker's API.
interface CycleTokenSnapshot {
  readonly has: boolean;
  readonly needsReauth: boolean;
}

function buildLiveGateDeps(
  killSwitches: KillSwitches,
  snapshot: Map<string, CycleTokenSnapshot>
): PolicyGateDeps {
  return {
    registry: createToolRegistry(),
    switches: killSwitches,
    hasConsent: (userId: string): boolean => snapshot.get(userId)?.has ?? false,
    hasOAuth: (userId: string, provider: string): boolean => {
      if (provider !== 'google') return false;
      const s = snapshot.get(userId);
      return !!s && s.has && !s.needsReauth;
    }
  };
}

/* ---------------------------------------------------------------------- */
/* Ranker bootstrap (Phase 3C.3)                                          */
/* ---------------------------------------------------------------------- */

// Build the ranker dep that the polling worker uses. Returns null when
// the kill switch is off (the safe default). THROWS when the kill
// switch is on but the OpenAI config is incomplete — "real or absent,
// never half-wired": if FOMO_RANKER_ENABLED=true we either deliver a
// working ranker or refuse to boot. Misconfig must surface at startup,
// not silently degrade to a no-op polling loop.
function buildRankerDep(
  storesHandle: SubstrateStoresHandle,
  killSwitches: KillSwitches,
  env: NodeJS.ProcessEnv = process.env
): GmailPollRankerDep | null {
  if (!killSwitches.ranker_enabled) return null;

  const apiKey = env.OPENAI_API_KEY;
  if (!apiKey || apiKey.length === 0) {
    throw new Error(
      'FOMO_RANKER_ENABLED=true but OPENAI_API_KEY is missing. ' +
        'Set the key or set FOMO_RANKER_ENABLED=false. Refusing to boot a half-wired ranker.'
    );
  }

  const model = env.FOMO_OPENAI_MODEL?.trim() || 'gpt-5-mini';
  const backend = new OpenAIBackend({
    apiKey,
    model,
    responseFormat: RANKER_OPENAI_RESPONSE_FORMAT
  });
  const router = createModelRouter({ costStore: storesHandle.stores.cost });
  router.registerBackend('classification', backend);

  return Object.freeze({
    rank: (req: Parameters<GmailPollRankerDep['rank']>[0]) => rankEmail(req, { router }),
    store: storesHandle.stores.rankResults
  });
}

function startGmailPolling(
  storesHandle: SubstrateStoresHandle,
  gmailClient: GmailClient,
  dispatch: DispatchTable,
  killSwitches: KillSwitches,
  config: FomoConfig,
  ranker: GmailPollRankerDep | null
): PollingHandle {
  const stores = storesHandle.stores;
  let stopped = false;
  let inflight: Promise<void> = Promise.resolve();
  let timer: ReturnType<typeof setTimeout> | null = null;
  let cyclesRun = 0;
  const cap = killSwitches.polling_max_cycles; // null = unbounded

  const tick = (): void => {
    if (stopped) return;
    inflight = (async () => {
      try {
        const userIds = await stores.gmailCursors.listUserIds();
        const snapshot = new Map<string, CycleTokenSnapshot>();
        for (const uid of userIds) {
          const tokens = await stores.tokens.list(uid);
          const google = tokens.find((t) => t.provider === 'google');
          snapshot.set(uid, {
            has: !!google,
            needsReauth: google?.needs_reauth ?? false
          });
        }
        const gateDeps = buildLiveGateDeps(killSwitches, snapshot);
        const report = await runPollOnce({
          gmailClient,
          tokenStore: stores.tokens,
          cursorStore: stores.gmailCursors,
          dispatch,
          auditStore: stores.audit,
          toolInvocationStore: stores.toolInvocations,
          gateDeps,
          ranker: ranker ?? undefined
        });
        cyclesRun++;
        if (stopped) return;
        logEvent(config, undefined, 'fomo.poll.cycle', 'INFO', {
          cycle_number: cyclesRun,
          cycle_cap: cap,
          users_total: report.users_total,
          users_polled: report.users_polled,
          users_skipped: report.users_skipped,
          users_unauthorized: report.users_unauthorized,
          users_api_error: report.users_api_error,
          messages_observed: report.messages_observed,
          messages_dispatched: report.messages_dispatched,
          messages_failed: report.messages_failed,
          // Phase 3C.3: only meaningful when ranker dep was built; zero
          // when ranker_enabled=false. Visible in the same log line so
          // ops can confirm the ranker is firing without correlating
          // audit rows.
          messages_ranked: report.messages_ranked,
          messages_rank_already: report.messages_rank_already,
          messages_rank_failed: report.messages_rank_failed
        });
      } catch (err) {
        if (stopped) return;
        logEvent(config, undefined, 'fomo.poll.error', 'ERROR', {
          error: err instanceof Error ? err.message : String(err)
        });
      }
    })();
    void inflight.finally(() => {
      if (stopped) return;
      // Phase 3B.3: bounded smoke test. When FOMO_GMAIL_POLLING_MAX_CYCLES
      // is set, auto-stop after that many cycles and emit one terminal
      // log event so ops can confirm the cap fired.
      if (cap !== null && cyclesRun >= cap) {
        stopped = true;
        logEvent(config, undefined, 'fomo.poll.cycle_cap_reached', 'INFO', {
          cycles_run: cyclesRun,
          cycle_cap: cap
        });
        return;
      }
      timer = setTimeout(tick, killSwitches.polling_interval_ms);
    });
  };

  tick();

  return {
    async stop() {
      stopped = true;
      if (timer !== null) {
        clearTimeout(timer);
        timer = null;
      }
      await inflight.catch(() => undefined);
    }
  };
}

export function createFomoRuntime(config: FomoConfig = loadFomoConfig()): FomoRuntime {
  const startedAtMs = Date.now();

  // Substrate stores — throws in production if BREVIO_TOKEN_KEK missing
  // and BREVIO_DEV_MODE is not 'true'. Same fail-closed behavior as the
  // Phase 2E client.
  const cryptoConfig = loadCryptoConfig();
  const storesHandle = createStores({ env: process.env, crypto: cryptoConfig });

  // Kill switches — read once at boot. Per FOMO_PLAN §16.5, defaults are
  // safe (everything off). FOMO_GMAIL_POLLING_ENABLED controls whether
  // the polling worker installs its interval.
  const killSwitches = loadKillSwitches(process.env);

  // Shared GmailClient — used by both the OAuth callback (to seed the
  // cursor at connect time) and the polling worker (to drive
  // listHistorySince + getMessage every cycle).
  const gmailClient = new GmailClient();

  // Dispatch table + executor wireup. Always wired regardless of polling
  // flag: an admin endpoint or ad-hoc caller could still invoke
  // gmail.read via dispatch when polling is off. The gate still gates
  // on consent + OAuth.
  const dispatch = createDispatchTable();
  wireInternalExecutors(dispatch, {
    audit: storesHandle.stores.audit,
    feedback: storesHandle.stores.feedback,
    memory: storesHandle.stores.memory
  });
  wireExternalExecutors(dispatch, {
    gmailClient,
    tokenStore: storesHandle.stores.tokens
  });

  // OAuth routes — graceful skip when not configured.
  const oauthGoogleDeps = buildOAuthGoogleDeps(storesHandle, config, gmailClient);

  // Ranker — bootstrapped only when FOMO_RANKER_ENABLED=true. THROWS at
  // boot if the kill switch is on but OpenAI config is incomplete; safe
  // default (kill switch off) returns null and the polling worker
  // behaves exactly as in 3B.2/3B.3.
  const ranker = buildRankerDep(storesHandle, killSwitches);
  if (ranker) {
    logEvent(config, undefined, 'fomo.ranker.enabled', 'INFO', {
      model: process.env.FOMO_OPENAI_MODEL?.trim() || 'gpt-5-mini',
      prompt_version_loaded: true
    });
  } else {
    logEvent(config, undefined, 'fomo.ranker.disabled', 'INFO', {
      detail: 'FOMO_RANKER_ENABLED is not "true"; ranker dormant (rank_results stays empty)'
    });
  }

  // Polling worker — bootstrapped only when FOMO_GMAIL_POLLING_ENABLED=true.
  // Safe default: off (no autonomous Gmail reads until founder opts in).
  let pollingHandle: PollingHandle | null = null;
  if (killSwitches.polling_enabled) {
    pollingHandle = startGmailPolling(storesHandle, gmailClient, dispatch, killSwitches, config, ranker);
    logEvent(config, undefined, 'fomo.poll.enabled', 'INFO', {
      interval_ms: killSwitches.polling_interval_ms,
      cycle_cap: killSwitches.polling_max_cycles,
      ranker_enabled: ranker !== null
    });
  } else {
    logEvent(config, undefined, 'fomo.poll.disabled', 'INFO', {
      detail: 'FOMO_GMAIL_POLLING_ENABLED is not "true"; polling worker dormant'
    });
  }

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
      if (pollingHandle) {
        await pollingHandle.stop();
      }
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
      oauth_google_wired: oauthGoogleDeps !== null,
      polling_enabled: killSwitches.polling_enabled,
      ranker_enabled: ranker !== null
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
