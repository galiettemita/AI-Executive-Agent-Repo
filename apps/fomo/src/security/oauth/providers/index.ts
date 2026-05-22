// Per-provider OAuth configuration. v0.1 supports Google only per
// FOMO_PLAN §9.2 ("v0.1 provider: Google Gmail read-only"). The token
// store and crypto primitives accept arbitrary provider strings for
// abstraction; this registry is the gateway to live OAuth flows.

export type OAuthProviderId = 'google';

export interface ProviderConfig {
  id: OAuthProviderId;
  authorizeUrl: string;
  tokenUrl: string;
  revokeUrl?: string;
  clientId: string;
  clientSecret: string;
  redirectUri: string;
  extraAuthorizeParams?: Record<string, string>;
}

export const SUPPORTED_PROVIDERS: OAuthProviderId[] = ['google'];

export function isSupportedProvider(value: unknown): value is OAuthProviderId {
  return typeof value === 'string' && (SUPPORTED_PROVIDERS as readonly string[]).includes(value);
}

interface ProviderEnvSpec {
  authorizeUrl: string;
  tokenUrl: string;
  revokeUrl?: string;
  clientIdEnv: string;
  clientSecretEnv: string;
  redirectUriEnv: string;
  extraAuthorizeParams?: Record<string, string>;
}

const PROVIDER_SPECS: Record<OAuthProviderId, ProviderEnvSpec> = {
  google: {
    authorizeUrl: 'https://accounts.google.com/o/oauth2/v2/auth',
    tokenUrl: 'https://oauth2.googleapis.com/token',
    revokeUrl: 'https://oauth2.googleapis.com/revoke',
    clientIdEnv: 'GOOGLE_CLIENT_ID',
    clientSecretEnv: 'GOOGLE_CLIENT_SECRET',
    redirectUriEnv: 'BREVIO_OAUTH_REDIRECT_URI_GOOGLE',
    extraAuthorizeParams: { access_type: 'offline', prompt: 'consent' }
  }
};

export function loadProviderConfig(id: OAuthProviderId): ProviderConfig | null {
  const spec = PROVIDER_SPECS[id];
  const clientId = process.env[spec.clientIdEnv]?.trim();
  const clientSecret = process.env[spec.clientSecretEnv]?.trim();
  const redirectUri = process.env[spec.redirectUriEnv]?.trim();
  if (!clientId || !clientSecret || !redirectUri) return null;
  return {
    id,
    authorizeUrl: spec.authorizeUrl,
    tokenUrl: spec.tokenUrl,
    revokeUrl: spec.revokeUrl,
    clientId,
    clientSecret,
    redirectUri,
    extraAuthorizeParams: spec.extraAuthorizeParams
  };
}

export function isProviderConfigured(id: OAuthProviderId): boolean {
  return loadProviderConfig(id) !== null;
}

export function buildAuthorizeUrl(
  config: ProviderConfig,
  scopes: string[],
  state: string,
  codeChallenge: string
): string {
  const params = new URLSearchParams({
    client_id: config.clientId,
    redirect_uri: config.redirectUri,
    response_type: 'code',
    scope: scopes.join(' '),
    state,
    code_challenge: codeChallenge,
    code_challenge_method: 'S256',
    ...(config.extraAuthorizeParams ?? {})
  });
  return `${config.authorizeUrl}?${params.toString()}`;
}
