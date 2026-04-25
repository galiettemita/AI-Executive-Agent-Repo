export interface OAuthService {
  service: string;
  provider_url: string;
  skills_using: string[];
  required_scopes: string[];
  token_type: string;
  refresh_strategy: string;
}

export interface APIKeyService {
  service: string;
  key_source: string;
  skills_using: string[];
  rate_limit: string;
  cost_model: string;
}

export interface NoAuthService {
  service: string;
  skills_using: string[];
  notes: string;
}

export interface AuthConfigStorage {
  secret_naming_convention: [string, string];
  oauth_redirect_uri_pattern: string;
  pkce_required: boolean;
}

export interface AuthServiceMap {
  oauth_services: OAuthService[];
  api_key_services: APIKeyService[];
  no_auth_services: NoAuthService[];
  auth_config_storage: AuthConfigStorage;
}

export interface EnvConfig {
  serviceName: string;
  serviceVersion: string;
  environment: string;
  port: number;
  mapPath: string;
  stateStoreFilePath?: string;
  internalAuthSecret: string;
  internalAuthIssuer: string;
  serviceAudience: string;
  callerContextSecret: string;
  logSalt: string;
  stateEncryptionSecret: string;
  completionRedirectAllowlist: Record<string, string[]>;
  tokenExchangeMode: 'simulated' | 'disabled';
  stateTtlMs: number;
  shutdownTimeoutMs: number;
}

export interface RequestContext {
  traceId: string;
  spanId: string;
  correlationId: string;
  subjectRef?: string;
}

export interface OAuthStateRecord {
  service: string;
  userId: string;
  completionRedirectUri?: string;
  codeVerifier: string;
  createdAtMs: number;
  expiresAtMs: number;
}

export interface OAuthAuthorizeRequest {
  redirect_uri?: string;
  scope_override?: string[];
}

export interface OAuthExchangeRequest {
  state: string;
  code: string;
}

export interface OAuthRefreshRequest {
  refresh_token: string;
}
