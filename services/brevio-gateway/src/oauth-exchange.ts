// OAuth code exchange + refresh + revoke. Uses fetch (Node 24 has it global).
// Pure functions; tests inject a mock fetch.

import type { ProviderConfig } from './oauth-providers/index.js';

export type FetchLike = typeof fetch;

export interface TokenResult {
  access_token: string;
  refresh_token?: string;
  token_type: string;
  expires_in?: number;
  scope?: string;
}

export interface ExchangeArgs {
  config: ProviderConfig;
  code: string;
  codeVerifier: string;
}

export interface RefreshArgs {
  config: ProviderConfig;
  refreshToken: string;
}

export class OAuthError extends Error {
  public readonly httpStatus: number;
  public readonly providerError: string | undefined;
  public readonly retryable: boolean;
  constructor(message: string, httpStatus: number, providerError: string | undefined, retryable: boolean) {
    super(message);
    this.httpStatus = httpStatus;
    this.providerError = providerError;
    this.retryable = retryable;
  }
}

function isRetryable(httpStatus: number, errorCode: string | undefined): boolean {
  if (errorCode === 'invalid_grant') return false;
  if (httpStatus >= 500) return true;
  if (httpStatus === 429) return true;
  return false;
}

async function postFormUrlencoded(
  url: string,
  body: Record<string, string>,
  fetchImpl: FetchLike
): Promise<Response> {
  const params = new URLSearchParams(body);
  return fetchImpl(url, {
    method: 'POST',
    headers: {
      'content-type': 'application/x-www-form-urlencoded',
      accept: 'application/json'
    },
    body: params.toString()
  });
}

async function parseTokenResult(res: Response): Promise<TokenResult> {
  let json: unknown;
  try {
    json = await res.json();
  } catch {
    throw new OAuthError(`provider returned non-JSON (status ${res.status})`, res.status, undefined, isRetryable(res.status, undefined));
  }
  if (!res.ok) {
    const obj = (json ?? {}) as Record<string, unknown>;
    const error = typeof obj.error === 'string' ? obj.error : undefined;
    const desc = typeof obj.error_description === 'string' ? obj.error_description : undefined;
    throw new OAuthError(
      `oauth provider error: ${error ?? 'unknown'} ${desc ?? ''}`.trim(),
      res.status,
      error,
      isRetryable(res.status, error)
    );
  }
  const obj = (json ?? {}) as Record<string, unknown>;
  if (typeof obj.access_token !== 'string') {
    throw new OAuthError('provider response missing access_token', res.status, undefined, false);
  }
  return {
    access_token: obj.access_token,
    refresh_token: typeof obj.refresh_token === 'string' ? obj.refresh_token : undefined,
    token_type: typeof obj.token_type === 'string' ? obj.token_type : 'Bearer',
    expires_in: typeof obj.expires_in === 'number' ? obj.expires_in : undefined,
    scope: typeof obj.scope === 'string' ? obj.scope : undefined
  };
}

export async function exchangeCodeForToken(args: ExchangeArgs, fetchImpl: FetchLike = fetch): Promise<TokenResult> {
  const res = await postFormUrlencoded(
    args.config.tokenUrl,
    {
      grant_type: 'authorization_code',
      code: args.code,
      redirect_uri: args.config.redirectUri,
      client_id: args.config.clientId,
      client_secret: args.config.clientSecret,
      code_verifier: args.codeVerifier
    },
    fetchImpl
  );
  return parseTokenResult(res);
}

export async function refreshAccessToken(args: RefreshArgs, fetchImpl: FetchLike = fetch): Promise<TokenResult> {
  const res = await postFormUrlencoded(
    args.config.tokenUrl,
    {
      grant_type: 'refresh_token',
      refresh_token: args.refreshToken,
      client_id: args.config.clientId,
      client_secret: args.config.clientSecret
    },
    fetchImpl
  );
  return parseTokenResult(res);
}

export async function revokeAtProvider(
  config: ProviderConfig,
  token: string,
  fetchImpl: FetchLike = fetch
): Promise<void> {
  if (!config.revokeUrl) return;
  const res = await postFormUrlencoded(
    config.revokeUrl,
    {
      token,
      client_id: config.clientId,
      client_secret: config.clientSecret
    },
    fetchImpl
  );
  if (!res.ok && res.status !== 400) {
    throw new OAuthError(`revoke failed: ${res.status}`, res.status, undefined, true);
  }
}
