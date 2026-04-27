import { createHash, randomUUID } from 'node:crypto';
import http from 'node:http';

import {
  extractBearerToken,
  hashTokenBinding,
  pseudonymizedRef,
  signAccessToken,
  verifyAccessToken,
  verifyCallerContextEnvelope
} from '../../../packages/shared/src/security.js';
import { loadAuthServiceMap, loadEnvConfig } from './config.js';
import { logJSON, requestContextFromHeaders } from './logger.js';
import { OAuthStateStore } from './oauth-state-store.js';
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
  stateStore: OAuthStateStore;
}

interface AuthenticatedRequestIdentity {
  subject: string;
  userId?: string;
  admin: boolean;
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

function resolveRedirectURI(map: AuthServiceMap, config: EnvConfig, service: string): string {
  return map.auth_config_storage.oauth_redirect_uri_pattern
    .replace('{service}', service)
    .replace('{environment}', config.environment);
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
    user_ref: createHash('sha256').update(record.userId).digest('hex').slice(0, 12),
    completion_redirect_configured: Boolean(record.completionRedirectUri),
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

function assertOAuthService(runtime: RuntimeState, service: string): OAuthService | undefined {
  return runtime.oauthByService.get(service);
}

function tokenHash(prefix: string, seed: string): string {
  return `${prefix}_${createHash('sha256').update(seed).digest('hex').slice(0, 40)}`;
}

function buildRuntimeState(config: EnvConfig, serviceMap: AuthServiceMap, stateStoreFilePath?: string): RuntimeState {
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
    stateStore: new OAuthStateStore(stateStoreFilePath, config.stateEncryptionSecret)
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

function authenticateRequest(
  req: http.IncomingMessage,
  config: EnvConfig,
  ctx: RequestContext,
  mode: 'api' | 'admin'
): AuthenticatedRequestIdentity {
  const token = extractBearerToken(req.headers.authorization);
  if (!token) {
    throw new Error('authorization_required');
  }
  const principal = verifyAccessToken(config.accessTokenIssuers, token, {
    expectedAudience: config.serviceAudience,
    allowedTokenUses: mode === 'admin' ? ['admin_access'] : ['service_access', 'admin_access', 'user_access']
  });
  ctx.subjectRef = pseudonymizedRef(principal.sub, config.logSalt);
  return {
    subject: principal.sub,
    userId: principal.token_use === 'user_access' || principal.token_use === 'admin_access' ? principal.sub : undefined,
    admin: principal.token_use === 'admin_access'
  };
}

function callerUserId(req: http.IncomingMessage, config: EnvConfig): string | undefined {
  const envelope = req.headers['x-brevio-caller-context'];
  const token = typeof envelope === 'string' ? envelope.trim() : Array.isArray(envelope) ? envelope[0]?.trim() : undefined;
  if (!token) {
    return undefined;
  }
  const bearerToken = extractBearerToken(getHeader(req, 'authorization'));
  if (!bearerToken) {
    throw new Error('authorization_required');
  }
  return verifyCallerContextEnvelope(config.callerContextIssuers, token, {
    expectedAudience: config.serviceAudience,
    expectedAccessTokenHash: hashTokenBinding(bearerToken)
  }).user_id;
}

function normalizeAllowlistedRedirect(urlValue: string): string {
  const url = new URL(urlValue);
  url.hash = '';
  return url.toString();
}

function resolveCompletionRedirect(
  config: EnvConfig,
  service: string,
  redirectUri: string | undefined
): string | undefined {
  const normalized = redirectUri?.trim();
  if (!normalized) {
    return undefined;
  }
  const allowlist = config.completionRedirectAllowlist[service] ?? config.completionRedirectAllowlist.default ?? [];
  const expected = normalizeAllowlistedRedirect(normalized);
  if (!allowlist.map((entry) => normalizeAllowlistedRedirect(entry)).includes(expected)) {
    throw new Error('redirect_uri_not_allowlisted');
  }
  return expected;
}

function tokenExchangeDisabledPayload(config: EnvConfig): Record<string, unknown> {
  return {
    error: 'token_exchange_not_configured',
    message: `oauth token exchange mode is ${config.tokenExchangeMode}; simulated exchange is only intended for development/test flows`
  };
}

async function handleAuthorize(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  config: EnvConfig,
  map: AuthServiceMap,
  runtime: RuntimeState,
  service: string,
  ctx: RequestContext,
  identity: AuthenticatedRequestIdentity
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

  const state = generateState();
  const codeVerifier = generateCodeVerifier();
  const codeChallenge = codeChallengeS256(codeVerifier);
  const createdAtMs = Date.now();
  const expiresAtMs = createdAtMs + config.stateTtlMs;
  let completionRedirectUri: string | undefined;
  try {
    completionRedirectUri = resolveCompletionRedirect(config, service, typeof body.redirect_uri === 'string' ? body.redirect_uri : undefined);
  } catch (error) {
    sendJSON(res, 400, { error: error instanceof Error ? error.message : 'redirect_uri_not_allowlisted' });
    return;
  }
  let userId: string | undefined;
  try {
    userId = callerUserId(req, config) ?? identity.userId;
  } catch (error) {
    sendJSON(res, 401, { error: error instanceof Error ? error.message : 'invalid_caller_context' });
    return;
  }
  if (!userId) {
    sendJSON(res, 400, { error: 'caller_context_required' });
    return;
  }

  const scopes =
    Array.isArray(body.scope_override) && body.scope_override.length > 0
      ? body.scope_override.join(' ')
      : oauthService.required_scopes.join(' ');

  runtime.stateStore.put(state, {
    service,
    userId,
    completionRedirectUri,
    codeVerifier,
    createdAtMs,
    expiresAtMs
  });

  const authURL = new URL(oauthService.provider_url);
  authURL.searchParams.set('response_type', 'code');
  authURL.searchParams.set('client_id', resolveClientID(config, map, service));
  authURL.searchParams.set('redirect_uri', resolveRedirectURI(map, config, service));
  authURL.searchParams.set('scope', scopes);
  authURL.searchParams.set('state', state);
  authURL.searchParams.set('code_challenge', codeChallenge);
  authURL.searchParams.set('code_challenge_method', 'S256');

  logJSON('oauth_authorize_created', 'INFO', config.serviceName, config.environment, ctx, {
    oauth_service: service,
    state_ref: createHash('sha256').update(state).digest('hex').slice(0, 12),
    user_ref: pseudonymizedRef(userId, config.logSalt),
    expires_at_ms: expiresAtMs,
    completion_redirect_configured: Boolean(completionRedirectUri)
  });

  sendJSON(res, 200, {
    service,
    provider_url: oauthService.provider_url,
    authorize_url: authURL.toString(),
    state,
    expires_at: new Date(expiresAtMs).toISOString(),
    pkce_required: map.auth_config_storage.pkce_required,
    completion_redirect_configured: Boolean(completionRedirectUri)
  });
}

function consumeState(runtime: RuntimeState, service: string, state: string): OAuthStateRecord | null {
  return runtime.stateStore.consume(service, state, Date.now());
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

  if (config.tokenExchangeMode !== 'simulated') {
    sendJSON(res, 503, tokenExchangeDisabledPayload(config));
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
    state_ref: createHash('sha256').update(body.state).digest('hex').slice(0, 12),
    user_ref: pseudonymizedRef(record.userId, config.logSalt),
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

  if (config.tokenExchangeMode !== 'simulated') {
    sendJSON(res, 503, tokenExchangeDisabledPayload(config));
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
      service: oauth
    });
    return;
  }

  const apiKey = runtime.apiKeyByService.get(service);
  if (apiKey) {
    sendJSON(res, 200, {
      type: 'api_key',
      service: apiKey
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
  config: EnvConfig,
  runtime: RuntimeState,
  service: string,
  state: string,
  code: string,
  issuerHint?: string
): void {
  const oauthService = assertOAuthService(runtime, service);
  if (!oauthService) {
    sendJSON(res, 404, { error: 'oauth_service_not_found', service });
    return;
  }
  if (issuerHint) {
    const expectedIssuer = new URL(oauthService.provider_url).origin;
    let actualIssuer: string;
    try {
      actualIssuer = new URL(issuerHint).origin;
    } catch {
      sendJSON(res, 400, { error: 'invalid_issuer_hint', service });
      return;
    }
    if (actualIssuer !== expectedIssuer) {
      sendJSON(res, 400, { error: 'issuer_mismatch', service });
      return;
    }
  }
  const record = consumeState(runtime, service, state);
  if (!record) {
    sendJSON(res, 409, {
      error: 'invalid_or_expired_state',
      service
    });
    return;
  }

  const nowMs = Date.now();
  const handoffToken = signAccessToken(config.userTokenSigningKey, {
    version: 2,
    sub: record.userId,
    iss: config.userTokenIssuer,
    aud: config.serviceAudience,
    iat: Math.floor(nowMs / 1000),
    exp: Math.floor((nowMs + 5 * 60 * 1000) / 1000),
    token_use: 'user_access',
    scopes: ['oauth:handoff']
  });
  if (record.completionRedirectUri) {
    const redirectTarget = new URL(record.completionRedirectUri);
    redirectTarget.searchParams.set('handoff_token', handoffToken);
    redirectTarget.searchParams.set('service', service);
    res.writeHead(303, { location: redirectTarget.toString() });
    res.end();
    return;
  }

  sendJSON(res, 200, {
    status: 'callback_processed',
    service,
    handoff_token: handoffToken,
    token_exchange_mode: config.tokenExchangeMode,
    state_record: scrubRecord(record)
  });
}

export function createAuthServiceRuntime(config?: EnvConfig, serviceMap?: AuthServiceMap): AuthServiceRuntime {
  const resolvedConfig = config ?? loadEnvConfig();
  const resolvedServiceMap = serviceMap ?? loadAuthServiceMap(resolvedConfig.mapPath);
  const runtime = buildRuntimeState(resolvedConfig, resolvedServiceMap, resolvedConfig.stateStoreFilePath);

  const cleanup = setInterval(() => {
    runtime.stateStore.expire(Date.now());
  }, 60000);
  cleanup.unref?.();

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
      try {
        authenticateRequest(req, resolvedConfig, ctx, 'admin');
      } catch (error) {
        onError(401, error instanceof Error ? error.message : 'authorization_required');
        return;
      }
      sendJSON(res, 200, {
        status: 'healthy',
        version: resolvedConfig.serviceVersion,
        uptime_ms: process.uptime() * 1000,
        checks: {
          auth_map_loaded: true,
          oauth_provider_count: runtime.oauthByService.size,
          api_key_provider_count: runtime.apiKeyByService.size,
          no_auth_provider_count: runtime.noAuthByService.size,
          oauth_state_store_mode: runtime.stateStore.mode(),
          oauth_state_file_path: runtime.stateStore.snapshotPath(),
          token_exchange_mode: resolvedConfig.tokenExchangeMode,
          active_oauth_states: runtime.stateStore.size()
        }
      });
      return;
    }

    if (req.method === 'GET' && segments.length === 3 && segments[0] === 'api' && segments[1] === 'v1' && segments[2] === 'providers') {
      try {
        authenticateRequest(req, resolvedConfig, ctx, 'admin');
      } catch (error) {
        onError(401, error instanceof Error ? error.message : 'authorization_required');
        return;
      }
      handleProviderList(res, resolvedServiceMap, runtime, resolvedConfig);
      return;
    }

    if (req.method === 'GET' && segments.length === 4 && segments[0] === 'api' && segments[1] === 'v1' && segments[2] === 'providers') {
      try {
        authenticateRequest(req, resolvedConfig, ctx, 'admin');
      } catch (error) {
        onError(401, error instanceof Error ? error.message : 'authorization_required');
        return;
      }
      const service = segments[3];
      if (!service) {
        onError(400, 'service_required');
        return;
      }
      handleProviderByService(res, resolvedServiceMap, runtime, resolvedConfig, service);
      return;
    }

    if (req.method === 'POST' && segments.length === 5 && segments[0] === 'api' && segments[1] === 'v1' && segments[2] === 'oauth' && segments[4] === 'authorize') {
      let identity: AuthenticatedRequestIdentity;
      try {
        identity = authenticateRequest(req, resolvedConfig, ctx, 'api');
      } catch (error) {
        onError(401, error instanceof Error ? error.message : 'authorization_required');
        return;
      }
      const service = segments[3];
      if (!service) {
        onError(400, 'service_required');
        return;
      }
      void handleAuthorize(req, res, resolvedConfig, resolvedServiceMap, runtime, service, ctx, identity);
      return;
    }

    if (req.method === 'POST' && segments.length === 5 && segments[0] === 'api' && segments[1] === 'v1' && segments[2] === 'oauth' && segments[4] === 'exchange') {
      try {
        authenticateRequest(req, resolvedConfig, ctx, 'api');
      } catch (error) {
        onError(401, error instanceof Error ? error.message : 'authorization_required');
        return;
      }
      const service = segments[3];
      if (!service) {
        onError(400, 'service_required');
        return;
      }
      void handleExchange(req, res, resolvedConfig, runtime, service, ctx);
      return;
    }

    if (req.method === 'POST' && segments.length === 5 && segments[0] === 'api' && segments[1] === 'v1' && segments[2] === 'oauth' && segments[4] === 'refresh') {
      try {
        authenticateRequest(req, resolvedConfig, ctx, 'api');
      } catch (error) {
        onError(401, error instanceof Error ? error.message : 'authorization_required');
        return;
      }
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

      handleCallback(res, resolvedConfig, runtime, service, state, code, url.searchParams.get('iss') ?? undefined);
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
