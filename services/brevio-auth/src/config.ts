import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  buildAccessTokenIssuerRegistry,
  buildCallerContextIssuerRegistry,
  loadBrevioEnvironment,
  requireSharedSecret,
  resolveAccessTokenSigningKey,
  resolveAccessTokenVerificationKey,
  resolveCallerContextVerificationKey
} from '../../../packages/shared/src/security.js';

import type { APIKeyService, AuthServiceMap, EnvConfig, NoAuthService, OAuthService } from './types.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

function parsePositiveInt(raw: string | undefined, fallback: number, field: string): number {
  if (!raw || raw.trim() === '') {
    return fallback;
  }
  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`invalid ${field}: must be positive integer`);
  }
  return parsed;
}

function parseTokenExchangeMode(raw: string | undefined, environment: string): EnvConfig['tokenExchangeMode'] {
  if (!raw || raw.trim() === '') {
    return environment === 'local' || environment === 'test' ? 'simulated' : 'disabled';
  }

  if (raw === 'simulated' || raw === 'disabled') {
    return raw;
  }

  throw new Error('invalid BREVIO_AUTH_TOKEN_EXCHANGE_MODE: expected simulated or disabled');
}

function parseRedirectAllowlist(raw: string | undefined): Record<string, string[]> {
  if (!raw || raw.trim() === '') {
    return {};
  }
  const parsed = JSON.parse(raw) as unknown;
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('BREVIO_AUTH_COMPLETION_REDIRECT_ALLOWLIST_JSON must be a JSON object');
  }
  const out: Record<string, string[]> = {};
  for (const [key, value] of Object.entries(parsed)) {
    if (!Array.isArray(value) || value.some((entry) => typeof entry !== 'string' || entry.trim() === '')) {
      throw new Error(`BREVIO_AUTH_COMPLETION_REDIRECT_ALLOWLIST_JSON.${key} must be a string array`);
    }
    out[key] = value.map((entry) => entry.trim());
  }
  return out;
}

function resolveMapPath(rawPath: string | undefined): string {
  if (rawPath && rawPath.trim() !== '') {
    return rawPath;
  }

  const candidates = [
    path.resolve(process.cwd(), 'config', 'auth-service-map.yaml'),
    path.resolve(process.cwd(), '..', '..', 'config', 'auth-service-map.yaml'),
    path.resolve(__dirname, '..', '..', '..', 'config', 'auth-service-map.yaml')
  ];

  for (const candidate of candidates) {
    try {
      readFileSync(candidate);
      return candidate;
    } catch {
      // try next path
    }
  }

  throw new Error('unable to resolve auth service map file path; set BREVIO_AUTH_MAP_PATH');
}

function isString(value: unknown): value is string {
  return typeof value === 'string' && value.trim() !== '';
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.length > 0 && value.every((entry) => isString(entry));
}

function normalizeScalar(raw: string): string {
  const trimmed = raw.trim();
  if (trimmed.startsWith('"') && trimmed.endsWith('"')) {
    return trimmed.slice(1, -1);
  }
  if (trimmed.startsWith("'") && trimmed.endsWith("'")) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

function parseInlineArray(raw: string): string[] {
  const trimmed = raw.trim();
  if (!trimmed.startsWith('[') || !trimmed.endsWith(']')) {
    return [normalizeScalar(trimmed)];
  }
  const inner = trimmed.slice(1, -1).trim();
  if (inner === '') {
    return [];
  }
  return inner
    .split(',')
    .map((entry) => normalizeScalar(entry))
    .filter((entry) => entry.length > 0);
}

function parseKeyValue(raw: string): [string, string] {
  const idx = raw.indexOf(':');
  if (idx <= 0) {
    throw new Error(`invalid yaml key/value line: ${raw}`);
  }
  const key = raw.slice(0, idx).trim();
  const value = raw.slice(idx + 1).trim();
  return [key, value];
}

function parseAuthServiceMapYAML(raw: string): AuthServiceMap {
  const oauthServices: OAuthService[] = [];
  const apiKeyServices: APIKeyService[] = [];
  const noAuthServices: NoAuthService[] = [];
  const secretNaming: string[] = [];
  let oauthRedirectURI = '';
  let pkceRequired = false;

  let section = '';
  let listMode = '';
  let currentItem: Record<string, string | string[]> | null = null;

  const flushCurrentItem = (): void => {
    if (!currentItem || listMode === '') {
      return;
    }

    const service = currentItem.service;
    if (!isString(service)) {
      throw new Error(`invalid ${listMode} item: missing service`);
    }

    if (listMode === 'oauth_services') {
      const item: OAuthService = {
        service,
        provider_url: asString(currentItem, 'provider_url', listMode),
        skills_using: asStringArray(currentItem, 'skills_using', listMode),
        required_scopes: asStringArray(currentItem, 'required_scopes', listMode),
        token_type: asString(currentItem, 'token_type', listMode),
        refresh_strategy: asString(currentItem, 'refresh_strategy', listMode)
      };
      oauthServices.push(item);
    }

    if (listMode === 'api_key_services') {
      const item: APIKeyService = {
        service,
        key_source: asString(currentItem, 'key_source', listMode),
        skills_using: asStringArray(currentItem, 'skills_using', listMode),
        rate_limit: asString(currentItem, 'rate_limit', listMode),
        cost_model: asString(currentItem, 'cost_model', listMode)
      };
      apiKeyServices.push(item);
    }

    if (listMode === 'no_auth_services') {
      const item: NoAuthService = {
        service,
        skills_using: asStringArray(currentItem, 'skills_using', listMode),
        notes: asString(currentItem, 'notes', listMode)
      };
      noAuthServices.push(item);
    }

    currentItem = null;
  };

  const lines = raw.split('\n');
  for (const line of lines) {
    if (line.trim() === '' || line.trimStart().startsWith('#')) {
      continue;
    }

    if (!line.startsWith(' ')) {
      flushCurrentItem();
      listMode = '';

      const [topKey] = parseKeyValue(line);
      section = topKey;
      continue;
    }

    if (section === 'oauth_services' || section === 'api_key_services' || section === 'no_auth_services') {
      listMode = section;
      const trimmed = line.trim();
      if (trimmed.startsWith('- ')) {
        flushCurrentItem();
        currentItem = {};
        const [key, value] = parseKeyValue(trimmed.slice(2));
        currentItem[key] = value.startsWith('[') ? parseInlineArray(value) : normalizeScalar(value);
        continue;
      }
      if (!currentItem) {
        throw new Error(`invalid ${listMode}: property line before list item`);
      }

      const [key, value] = parseKeyValue(trimmed);
      currentItem[key] = value.startsWith('[') ? parseInlineArray(value) : normalizeScalar(value);
      continue;
    }

    if (section === 'auth_config_storage') {
      const trimmed = line.trim();
      if (trimmed === 'secret_naming_convention:') {
        listMode = 'secret_naming_convention';
        continue;
      }
      if (listMode === 'secret_naming_convention' && trimmed.startsWith('- ')) {
        secretNaming.push(normalizeScalar(trimmed.slice(2)));
        continue;
      }

      const [key, value] = parseKeyValue(trimmed);
      if (key === 'oauth_redirect_uri_pattern') {
        oauthRedirectURI = normalizeScalar(value);
      }
      if (key === 'pkce_required') {
        pkceRequired = normalizeScalar(value) === 'true';
      }
      continue;
    }
  }

  flushCurrentItem();

  const parsed: AuthServiceMap = {
    oauth_services: oauthServices,
    api_key_services: apiKeyServices,
    no_auth_services: noAuthServices,
    auth_config_storage: {
      secret_naming_convention: [secretNaming[0] ?? '', secretNaming[1] ?? ''],
      oauth_redirect_uri_pattern: oauthRedirectURI,
      pkce_required: pkceRequired
    }
  };

  validateAuthServiceMap(parsed);
  return parsed;
}

function asString(
  item: Record<string, string | string[]>,
  key: string,
  section: string
): string {
  const value = item[key];
  if (!isString(value)) {
    throw new Error(`invalid ${section} item: ${key} must be non-empty string`);
  }
  return value;
}

function asStringArray(
  item: Record<string, string | string[]>,
  key: string,
  section: string
): string[] {
  const value = item[key];
  if (!isStringArray(value)) {
    throw new Error(`invalid ${section} item: ${key} must be non-empty string array`);
  }
  return value;
}

function validateAuthServiceMap(parsed: AuthServiceMap): void {
  if (parsed.oauth_services.length !== 15) {
    throw new Error('oauth_services must contain exactly 15 items');
  }
  if (parsed.api_key_services.length !== 18) {
    throw new Error('api_key_services must contain exactly 18 items');
  }
  if (parsed.no_auth_services.length !== 6) {
    throw new Error('no_auth_services must contain exactly 6 items');
  }

  for (const item of parsed.oauth_services) {
    if (
      !isString(item.service) ||
      !isString(item.provider_url) ||
      !isStringArray(item.skills_using) ||
      !isStringArray(item.required_scopes) ||
      !isString(item.token_type) ||
      !isString(item.refresh_strategy)
    ) {
      throw new Error(`invalid oauth service item: ${item.service}`);
    }
  }

  for (const item of parsed.api_key_services) {
    if (
      !isString(item.service) ||
      !isString(item.key_source) ||
      !isStringArray(item.skills_using) ||
      !isString(item.rate_limit) ||
      !isString(item.cost_model)
    ) {
      throw new Error(`invalid api key service item: ${item.service}`);
    }
  }

  for (const item of parsed.no_auth_services) {
    if (!isString(item.service) || !isStringArray(item.skills_using) || !isString(item.notes)) {
      throw new Error(`invalid no auth service item: ${item.service}`);
    }
  }

  if (parsed.auth_config_storage.secret_naming_convention.length !== 2) {
    throw new Error('auth_config_storage.secret_naming_convention must have exactly 2 items');
  }
  if (!isString(parsed.auth_config_storage.secret_naming_convention[0])) {
    throw new Error('auth_config_storage secret naming client_id entry must be non-empty');
  }
  if (!isString(parsed.auth_config_storage.secret_naming_convention[1])) {
    throw new Error('auth_config_storage secret naming client_secret entry must be non-empty');
  }
  if (!isString(parsed.auth_config_storage.oauth_redirect_uri_pattern)) {
    throw new Error('auth_config_storage.oauth_redirect_uri_pattern must be set');
  }
  if (parsed.auth_config_storage.pkce_required !== true) {
    throw new Error('auth_config_storage.pkce_required must be true');
  }
}

export function loadEnvConfig(): EnvConfig {
  const mapPath = resolveMapPath(process.env.BREVIO_AUTH_MAP_PATH);
  const environment = loadBrevioEnvironment();

  return {
    serviceName: 'brevio-auth',
    serviceVersion: process.env.SERVICE_VERSION ?? '0.2.0',
    environment,
    port: parsePositiveInt(process.env.PORT, 8080, 'PORT'),
    mapPath,
    stateStoreFilePath: path.resolve(process.env.BREVIO_AUTH_STATE_STORE_FILE ?? path.join(process.cwd(), 'data', 'auth', 'oauth-state.json')),
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
    userTokenSigningKey: resolveAccessTokenSigningKey(
      process.env.BREVIO_AUTH_ACCESS_PRIVATE_KEY,
      undefined,
      environment,
      'BREVIO_AUTH_ACCESS_PRIVATE_KEY',
      'auth-access'
    ),
    userTokenIssuer: process.env.BREVIO_AUTH_ACCESS_ISSUER?.trim() || 'https://auth.brevio.internal',
    serviceAudience: process.env.BREVIO_AUTH_AUDIENCE?.trim() || 'brevio-auth',
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
    logSalt: process.env.BREVIO_AUTH_LOG_SALT?.trim() || `brevio-auth:${environment}`,
    stateEncryptionSecret: requireSharedSecret(process.env.BREVIO_AUTH_STATE_ENCRYPTION_SECRET, 'BREVIO_AUTH_STATE_ENCRYPTION_SECRET', environment, 'brevio-auth-state'),
    completionRedirectAllowlist: parseRedirectAllowlist(process.env.BREVIO_AUTH_COMPLETION_REDIRECT_ALLOWLIST_JSON),
    tokenExchangeMode: parseTokenExchangeMode(process.env.BREVIO_AUTH_TOKEN_EXCHANGE_MODE, environment),
    stateTtlMs: parsePositiveInt(process.env.BREVIO_AUTH_STATE_TTL_MS, 600000, 'BREVIO_AUTH_STATE_TTL_MS'),
    shutdownTimeoutMs: parsePositiveInt(
      process.env.BREVIO_AUTH_SHUTDOWN_TIMEOUT_MS,
      30000,
      'BREVIO_AUTH_SHUTDOWN_TIMEOUT_MS'
    )
  };
}

export function loadAuthServiceMap(mapPath: string): AuthServiceMap {
  const raw = readFileSync(mapPath, 'utf8');
  return parseAuthServiceMapYAML(raw);
}
