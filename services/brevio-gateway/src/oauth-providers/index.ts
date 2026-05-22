// Per-provider OAuth configuration. Defines authorize/token/revoke URLs,
// client ID/secret env vars, and supported providers list.

export type OAuthProviderId = 'google' | 'microsoft' | 'apple' | 'spotify' | 'github' | 'notion';

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

export const SUPPORTED_PROVIDERS: OAuthProviderId[] = ['google', 'microsoft', 'apple', 'spotify', 'github', 'notion'];

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
  },
  microsoft: {
    authorizeUrl: 'https://login.microsoftonline.com/common/oauth2/v2.0/authorize',
    tokenUrl: 'https://login.microsoftonline.com/common/oauth2/v2.0/token',
    clientIdEnv: 'MICROSOFT_CLIENT_ID',
    clientSecretEnv: 'MICROSOFT_CLIENT_SECRET',
    redirectUriEnv: 'BREVIO_OAUTH_REDIRECT_URI_MICROSOFT'
  },
  apple: {
    authorizeUrl: 'https://appleid.apple.com/auth/authorize',
    tokenUrl: 'https://appleid.apple.com/auth/token',
    clientIdEnv: 'APPLE_CLIENT_ID',
    clientSecretEnv: 'APPLE_CLIENT_SECRET',
    redirectUriEnv: 'BREVIO_OAUTH_REDIRECT_URI_APPLE'
  },
  spotify: {
    authorizeUrl: 'https://accounts.spotify.com/authorize',
    tokenUrl: 'https://accounts.spotify.com/api/token',
    clientIdEnv: 'SPOTIFY_CLIENT_ID',
    clientSecretEnv: 'SPOTIFY_CLIENT_SECRET',
    redirectUriEnv: 'BREVIO_OAUTH_REDIRECT_URI_SPOTIFY'
  },
  github: {
    authorizeUrl: 'https://github.com/login/oauth/authorize',
    tokenUrl: 'https://github.com/login/oauth/access_token',
    clientIdEnv: 'GITHUB_CLIENT_ID',
    clientSecretEnv: 'GITHUB_CLIENT_SECRET',
    redirectUriEnv: 'BREVIO_OAUTH_REDIRECT_URI_GITHUB'
  },
  notion: {
    authorizeUrl: 'https://api.notion.com/v1/oauth/authorize',
    tokenUrl: 'https://api.notion.com/v1/oauth/token',
    clientIdEnv: 'NOTION_CLIENT_ID',
    clientSecretEnv: 'NOTION_CLIENT_SECRET',
    redirectUriEnv: 'BREVIO_OAUTH_REDIRECT_URI_NOTION'
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
