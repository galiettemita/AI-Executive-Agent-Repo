import { createHash, randomUUID } from 'node:crypto';
import http from 'node:http';

import { loadAuthServiceMap, loadEnvConfig } from './config.js';
import { logJSON, requestContextFromHeaders } from './logger.js';
import { codeChallengeS256, generateCodeVerifier, generateState } from './pkce.js';
import type {
  APIKeyService,
  AuthServiceMap,
  EnvConfig,
  NoAuthService,
  OAuthAuthorizeRequest,
  OAuthExchangeRequest,
  OAuthRefreshRequest,
  OAuthService,
  OAuthStateRecord,
  RequestContext
} from './types.js';

interface RuntimeState {
  oauthByService: Map<string, OAuthService>;
  apiKeyByService: Map<string, APIKeyService>;
  noAuthByService: Map<string, NoAuthService>;
  stateStore: Map<string, OAuthStateRecord>;
}

export interface AuthServiceRuntime {
  readonly config: EnvConfig;
  readonly serviceMap: AuthServiceMap;
  readonly server: http.Server;
  close(): Promise<void>;
}

function sendJSON(res: http.ServerResponse, statusCode: number, payload: Record<string, unknown>): void {
  res.writeHead(statusCode, { 'content-type': 'application/json' });
  res.end(JSON.stringify(payload));
}

function pathSegments(urlPath: string): string[] {
  return urlPath.split('/').filter((segment) => segment.length > 0);
}

function envVarForServiceClientID(service: string): string {
  return `OAUTH_CLIENT_ID_${service.replace(/[^a-zA-Z0-9]/g, '_').toUpperCase()}`;
}

function resolveRedirectURI(map: AuthServiceMap, service: string): string {
  return map.auth_config_storage.oauth_redirect_uri_pattern
    .replace('{service}', service)
    .replace('{environment}', process.env.NODE_ENV ?? 'development');
}

function resolveClientID(config: EnvConfig, map: AuthServiceMap, service: string): string {
  const envVar = envVarForServiceClientID(service);
  const value = process.env[envVar];
  if (value && value.trim() !== '') {
    return value;
  }
  const template = map.auth_config_storage.secret_naming_convention[0];
  const path = template.replace('{environment}', config.environment).replace('{service}', service);
  return `aws-secretsmanager://${path}`;
}

function scrubRecord(record: OAuthStateRecord): Record<string, unknown> {
  return {
    service: record.service,
    user_id: record.userId,
    redirect_uri: record.redirectUri,
    created_at_ms: record.createdAtMs,
    expires_at_ms: record.expiresAtMs
  };
}

async function parseBody<T>(req: http.IncomingMessage, maxBytes = 1024 * 1024): Promise<T> {
  return await new Promise<T>((resolve, reject) => {
    const chunks: Buffer[] = [];
    let received = 0;

    req.on('data', (chunk) => {
      const data = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
      received += data.byteLength;
      if (received > maxBytes) {
        reject(new Error('request_body_too_large'));
        req.destroy();
        return;
      }
      chunks.push(data);
    });

    req.on('error', (err) => reject(err));
    req.on('end', () => {
      if (chunks.length === 0) {
        resolve({} as unknown as T);
        return;
      }
      try {
        const parsed = JSON.parse(Buffer.concat(chunks).toString('utf8')) as unknown as T;
        resolve(parsed);
      } catch {
        reject(new Error('invalid_json'));
      }
    });
  });
}

function expireStateEntries(stateStore: Map<string, OAuthStateRecord>, nowMs: number): void {
  for (const [state, record] of stateStore.entries()) {
    if (record.expiresAtMs <= nowMs) {
      stateStore.delete(state);
    }
  }
}

function assertOAuthService(runtime: RuntimeState, service: string): OAuthService | undefined {
  return runtime.oauthByService.get(service);
}

function tokenHash(prefix: string, seed: string): string {
  return `${prefix}_${createHash('sha256').update(seed).digest('hex').slice(0, 40)}`;
}

function buildRuntimeState(serviceMap: AuthServiceMap): RuntimeState {
  const oauthByService = new Map<string, OAuthService>();
  for (const service of serviceMap.oauth_services) {
    oauthByService.set(service.service, service);
  }

  const apiKeyByService = new Map<string, APIKeyService>();
  for (const service of serviceMap.api_key_services) {
    apiKeyByService.set(service.service, service);
  }

  const noAuthByService = new Map<string, NoAuthService>();
  for (const service of serviceMap.no_auth_services) {
    noAuthByService.set(service.service, service);
  }

  return {
    oauthByService,
    apiKeyByService,
    noAuthByService,
    stateStore: new Map<string, OAuthStateRecord>()
  };
}

function simulateTokenExchange(
  service: string,
  userId: string,
  code: string,
  refreshTokenSeed: string
): Record<string, unknown> {
  const nowMs = Date.now();
  const expiresIn = 3600;
  const refreshExpiresIn = 30 * 24 * 3600;

  return {
    service,
    user_id: userId,
    access_token: tokenHash('access', `${service}:${code}:${nowMs}`),
    refresh_token: tokenHash('refresh', `${service}:${refreshTokenSeed}:${nowMs}`),
    token_type: 'Bearer',
    expires_in: expiresIn,
    refresh_expires_in: refreshExpiresIn,
    expires_at: new Date(nowMs + expiresIn * 1000).toISOString(),
    refresh_expires_at: new Date(nowMs + refreshExpiresIn * 1000).toISOString()
  };
}

function requestContext(req: http.IncomingMessage): RequestContext {
  return requestContextFromHeaders(req.headers);
}

async function handleAuthorize(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  config: EnvConfig,
  map: AuthServiceMap,
  runtime: RuntimeState,
  service: string,
  ctx: RequestContext
): Promise<void> {
  const oauthService = assertOAuthService(runtime, service);
  if (!oauthService) {
    sendJSON(res, 404, { error: 'oauth_service_not_found', service });
    return;
  }

  let body: OAuthAuthorizeRequest;
  try {
    body = await parseBody<OAuthAuthorizeRequest>(req);
  } catch (err) {
    sendJSON(res, 400, { error: err instanceof Error ? err.message : 'invalid_request_body' });
    return;
  }

  if (typeof body.user_id !== 'string' || body.user_id.trim() === '') {
    sendJSON(res, 400, { error: 'user_id_required' });
    return;
  }

  const state = generateState();
  const codeVerifier = generateCodeVerifier();
  const codeChallenge = codeChallengeS256(codeVerifier);
  const createdAtMs = Date.now();
  const expiresAtMs = createdAtMs + config.stateTtlMs;
  const redirectUri =
    typeof body.redirect_uri === 'string' && body.redirect_uri.trim() !== ''
      ? body.redirect_uri
      : resolveRedirectURI(map, service);

  const scopes =
    Array.isArray(body.scope_override) && body.scope_override.length > 0
      ? body.scope_override.join(' ')
      : oauthService.required_scopes.join(' ');

  runtime.stateStore.set(state, {
    service,
    userId: body.user_id,
    redirectUri,
    codeVerifier,
    createdAtMs,
    expiresAtMs
  });

  const authURL = new URL(oauthService.provider_url);
  authURL.searchParams.set('response_type', 'code');
  authURL.searchParams.set('client_id', resolveClientID(config, map, service));
  authURL.searchParams.set('redirect_uri', redirectUri);
  authURL.searchParams.set('scope', scopes);
  authURL.searchParams.set('state', state);
  authURL.searchParams.set('code_challenge', codeChallenge);
  authURL.searchParams.set('code_challenge_method', 'S256');

  logJSON('oauth_authorize_created', 'INFO', config.serviceName, config.environment, ctx, {
    oauth_service: service,
    state,
    user_id: body.user_id,
    expires_at_ms: expiresAtMs,
    redirect_uri: redirectUri
  });

  sendJSON(res, 200, {
    service,
    provider_url: oauthService.provider_url,
    authorize_url: authURL.toString(),
    state,
    expires_at: new Date(expiresAtMs).toISOString(),
    pkce_required: map.auth_config_storage.pkce_required
  });
}

function consumeState(runtime: RuntimeState, service: string, state: string): OAuthStateRecord | null {
  const record = runtime.stateStore.get(state);
  if (!record) {
    return null;
  }
  if (record.service !== service) {
    return null;
  }
  if (record.expiresAtMs <= Date.now()) {
    runtime.stateStore.delete(state);
    return null;
  }
  runtime.stateStore.delete(state);
  return record;
}

async function handleExchange(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  config: EnvConfig,
  runtime: RuntimeState,
  service: string,
  ctx: RequestContext
): Promise<void> {
  if (!assertOAuthService(runtime, service)) {
    sendJSON(res, 404, { error: 'oauth_service_not_found', service });
    return;
  }

  let body: OAuthExchangeRequest;
  try {
    body = await parseBody<OAuthExchangeRequest>(req);
  } catch (err) {
    sendJSON(res, 400, { error: err instanceof Error ? err.message : 'invalid_request_body' });
    return;
  }

  if (typeof body.state !== 'string' || body.state.trim() === '') {
    sendJSON(res, 400, { error: 'state_required' });
    return;
  }
  if (typeof body.code !== 'string' || body.code.trim() === '') {
    sendJSON(res, 400, { error: 'code_required' });
    return;
  }

  const record = consumeState(runtime, service, body.state);
  if (!record) {
    sendJSON(res, 409, { error: 'invalid_or_expired_state', service });
    return;
  }

  const token = simulateTokenExchange(service, record.userId, body.code, record.codeVerifier);
  logJSON('oauth_token_exchanged', 'INFO', config.serviceName, config.environment, ctx, {
    oauth_service: service,
    state: body.state,
    user_id: record.userId,
    state_record: scrubRecord(record)
  });

  sendJSON(res, 200, {
    status: 'success',
    token
  });
}

async function handleRefresh(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  config: EnvConfig,
  runtime: RuntimeState,
  service: string,
  ctx: RequestContext
): Promise<void> {
  if (!assertOAuthService(runtime, service)) {
    sendJSON(res, 404, { error: 'oauth_service_not_found', service });
    return;
  }

  let body: OAuthRefreshRequest;
  try {
    body = await parseBody<OAuthRefreshRequest>(req);
  } catch (err) {
    sendJSON(res, 400, { error: err instanceof Error ? err.message : 'invalid_request_body' });
    return;
  }

  if (typeof body.refresh_token !== 'string' || body.refresh_token.trim() === '') {
    sendJSON(res, 400, { error: 'refresh_token_required' });
    return;
  }

  const nowMs = Date.now();
  const expiresIn = 3600;
  const accessToken = tokenHash('access', `${service}:${body.refresh_token}:${nowMs}`);

  logJSON('oauth_token_refreshed', 'INFO', config.serviceName, config.environment, ctx, {
    oauth_service: service
  });

  sendJSON(res, 200, {
    status: 'success',
    service,
    access_token: accessToken,
    token_type: 'Bearer',
    expires_in: expiresIn,
    expires_at: new Date(nowMs + expiresIn * 1000).toISOString()
  });
}

function handleProviderList(
  res: http.ServerResponse,
  map: AuthServiceMap,
  runtime: RuntimeState,
  config: EnvConfig
): void {
  sendJSON(res, 200, {
    oauth_services: map.oauth_services,
    api_key_services: map.api_key_services,
    no_auth_services: map.no_auth_services,
    auth_config_storage: map.auth_config_storage,
    stats: {
      oauth_services: runtime.oauthByService.size,
      api_key_services: runtime.apiKeyByService.size,
      no_auth_services: runtime.noAuthByService.size
    },
    environment: config.environment
  });
}

function handleProviderByService(
  res: http.ServerResponse,
  map: AuthServiceMap,
  runtime: RuntimeState,
  config: EnvConfig,
  service: string
): void {
  const oauth = runtime.oauthByService.get(service);
  if (oauth) {
    sendJSON(res, 200, {
      type: 'oauth',
      service: oauth,
      secrets: {
        client_id: map.auth_config_storage.secret_naming_convention[0]
          .replace('{environment}', config.environment)
          .replace('{service}', service),
        client_secret: map.auth_config_storage.secret_naming_convention[1]
          .replace('{environment}', config.environment)
          .replace('{service}', service)
      }
    });
    return;
  }

  const apiKey = runtime.apiKeyByService.get(service);
  if (apiKey) {
    sendJSON(res, 200, {
      type: 'api_key',
      service: apiKey,
      secret_path: `brevio/${config.environment}/${service}/api_key`
    });
    return;
  }

  const noAuth = runtime.noAuthByService.get(service);
  if (noAuth) {
    sendJSON(res, 200, {
      type: 'no_auth',
      service: noAuth
    });
    return;
  }

  sendJSON(res, 404, { error: 'service_not_found', service });
}

function handleCallback(
  res: http.ServerResponse,
  runtime: RuntimeState,
  service: string,
  state: string,
  code: string
): void {
  const record = consumeState(runtime, service, state);
  if (!record) {
    sendJSON(res, 409, {
      error: 'invalid_or_expired_state',
      service
    });
    return;
  }

  sendJSON(res, 200, {
    status: 'callback_received',
    service,
    state,
    token_exchange_hint: {
      endpoint: `/api/v1/oauth/${service}/exchange`,
      body: {
        state,
        code
      }
    },
    state_record: scrubRecord(record)
  });
}

export function createAuthServiceRuntime(config?: EnvConfig, serviceMap?: AuthServiceMap): AuthServiceRuntime {
  const resolvedConfig = config ?? loadEnvConfig();
  const resolvedServiceMap = serviceMap ?? loadAuthServiceMap(resolvedConfig.mapPath);
  const runtime = buildRuntimeState(resolvedServiceMap);

  const cleanup = setInterval(() => {
    expireStateEntries(runtime.stateStore, Date.now());
  }, 60000);

  const server = http.createServer((req, res) => {
    const ctx = requestContext(req);
    const url = new URL(req.url ?? '/', 'http://localhost');
    const segments = pathSegments(url.pathname);

    const onError = (statusCode: number, code: string, detail?: unknown): void => {
      logJSON('request_failed', 'WARN', resolvedConfig.serviceName, resolvedConfig.environment, ctx, {
        method: req.method,
        path: url.pathname,
        code,
        detail
      });
      sendJSON(res, statusCode, { error: code });
    };

    if (req.method === 'GET' && url.pathname === '/health') {
      sendJSON(res, 200, {
        status: 'healthy',
        version: resolvedConfig.serviceVersion,
        uptime_ms: process.uptime() * 1000
      });
      return;
    }

    if (req.method === 'GET' && url.pathname === '/health/deep') {
      sendJSON(res, 200, {
        status: 'healthy',
        version: resolvedConfig.serviceVersion,
        uptime_ms: process.uptime() * 1000,
        checks: {
          auth_map_loaded: true,
          oauth_provider_count: runtime.oauthByService.size,
          api_key_provider_count: runtime.apiKeyByService.size,
          no_auth_provider_count: runtime.noAuthByService.size,
          active_oauth_states: runtime.stateStore.size
        }
      });
      return;
    }

    if (req.method === 'GET' && segments.length === 3 && segments[0] === 'api' && segments[1] === 'v1' && segments[2] === 'providers') {
      handleProviderList(res, resolvedServiceMap, runtime, resolvedConfig);
      return;
    }

    if (req.method === 'GET' && segments.length === 4 && segments[0] === 'api' && segments[1] === 'v1' && segments[2] === 'providers') {
      const service = segments[3];
      if (!service) {
        onError(400, 'service_required');
        return;
      }
      handleProviderByService(res, resolvedServiceMap, runtime, resolvedConfig, service);
      return;
    }

    if (req.method === 'POST' && segments.length === 5 && segments[0] === 'api' && segments[1] === 'v1' && segments[2] === 'oauth' && segments[4] === 'authorize') {
      const service = segments[3];
      if (!service) {
        onError(400, 'service_required');
        return;
      }
      void handleAuthorize(req, res, resolvedConfig, resolvedServiceMap, runtime, service, ctx);
      return;
    }

    if (req.method === 'POST' && segments.length === 5 && segments[0] === 'api' && segments[1] === 'v1' && segments[2] === 'oauth' && segments[4] === 'exchange') {
      const service = segments[3];
      if (!service) {
        onError(400, 'service_required');
        return;
      }
      void handleExchange(req, res, resolvedConfig, runtime, service, ctx);
      return;
    }

    if (req.method === 'POST' && segments.length === 5 && segments[0] === 'api' && segments[1] === 'v1' && segments[2] === 'oauth' && segments[4] === 'refresh') {
      const service = segments[3];
      if (!service) {
        onError(400, 'service_required');
        return;
      }
      void handleRefresh(req, res, resolvedConfig, runtime, service, ctx);
      return;
    }

    if (req.method === 'GET' && segments.length === 2 && segments[0] === 'callback') {
      const service = segments[1];
      if (!service) {
        onError(400, 'service_required');
        return;
      }
      const state = url.searchParams.get('state');
      const code = url.searchParams.get('code');
      const error = url.searchParams.get('error');

      if (error) {
        onError(400, 'provider_error', error);
        return;
      }
      if (!state || !code) {
        onError(400, 'state_and_code_required');
        return;
      }

      handleCallback(res, runtime, service, state, code);
      return;
    }

    onError(404, 'not_found');
  });

  return {
    config: resolvedConfig,
    serviceMap: resolvedServiceMap,
    server,
    async close(): Promise<void> {
      clearInterval(cleanup);
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
}

export async function startAuthService(): Promise<AuthServiceRuntime> {
  const runtime = createAuthServiceRuntime();
  await new Promise<void>((resolve, reject) => {
    runtime.server.listen(runtime.config.port, () => resolve());
    runtime.server.once('error', (err) => reject(err));
  });
  return runtime;
}

function processContext(): RequestContext {
  return {
    traceId: randomUUID(),
    spanId: randomUUID(),
    correlationId: randomUUID()
  };
}

export function installSignalHandlers(runtime: AuthServiceRuntime): void {
  const stop = async (signal: string): Promise<void> => {
    const ctx = processContext();
    logJSON('shutdown_start', 'INFO', runtime.config.serviceName, runtime.config.environment, ctx, { signal });

    const timeout = setTimeout(() => {
      logJSON('shutdown_timeout', 'ERROR', runtime.config.serviceName, runtime.config.environment, ctx, {
        timeout_ms: runtime.config.shutdownTimeoutMs
      });
      process.exit(1);
    }, runtime.config.shutdownTimeoutMs);

    try {
      await runtime.close();
      clearTimeout(timeout);
      logJSON('shutdown_complete', 'INFO', runtime.config.serviceName, runtime.config.environment, ctx, {});
      process.exit(0);
    } catch (err) {
      clearTimeout(timeout);
      logJSON('shutdown_failed', 'ERROR', runtime.config.serviceName, runtime.config.environment, ctx, {
        error: err instanceof Error ? err.message : String(err)
      });
      process.exit(1);
    }
  };

  process.on('SIGTERM', () => {
    void stop('SIGTERM');
  });
  process.on('SIGINT', () => {
    void stop('SIGINT');
  });
}
