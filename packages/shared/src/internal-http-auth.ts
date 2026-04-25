import type http from 'node:http';

import {
  extractBearerToken,
  extractHeaderValue,
  pseudonymizedRef,
  type AccessTokenUse,
  type VerifiedAccessToken,
  type VerifiedCallerContext,
  verifyAccessToken,
  verifyCallerContextEnvelope
} from './security.js';

export interface InternalHttpAuthConfig {
  internalAuthSecret: string;
  internalAuthIssuer: string;
  serviceAudience: string;
  callerContextSecret?: string;
  logSalt: string;
}

export interface InternalHttpRequestContextLike {
  subjectRef?: string;
}

export interface AuthenticatedInternalRequest {
  principal: VerifiedAccessToken;
  callerContext?: VerifiedCallerContext;
}

export interface AuthenticateInternalRequestOptions {
  mode?: 'api' | 'admin';
  allowedTokenUses?: AccessTokenUse[];
  requireCallerContext?: boolean;
}

export function authenticateInternalRequest(
  req: http.IncomingMessage,
  config: InternalHttpAuthConfig,
  ctx: InternalHttpRequestContextLike,
  options: AuthenticateInternalRequestOptions = {}
): AuthenticatedInternalRequest {
  const token = extractBearerToken(extractHeaderValue(req.headers, 'authorization'));
  if (!token) {
    throw new Error(options.mode === 'admin' ? 'admin_token_required' : 'authorization_required');
  }
  const allowedTokenUses =
    options.allowedTokenUses ??
    (options.mode === 'admin' ? ['admin_access'] : ['service_access', 'admin_access', 'user_access']);
  const principal = verifyAccessToken(config.internalAuthSecret, token, {
    expectedAudience: config.serviceAudience,
    expectedIssuer: config.internalAuthIssuer,
    allowedTokenUses
  });
  ctx.subjectRef = pseudonymizedRef(principal.sub, config.logSalt);

  const callerContextToken = extractHeaderValue(req.headers, 'x-brevio-caller-context');
  const callerContext =
    callerContextToken && config.callerContextSecret
      ? verifyCallerContextEnvelope(config.callerContextSecret, callerContextToken)
      : undefined;
  if (options.requireCallerContext && !callerContext) {
    throw new Error('caller_context_required');
  }
  return {
    principal,
    callerContext
  };
}

export function resolveEffectiveUserScope(
  auth: AuthenticatedInternalRequest,
  options: { requireUserId?: boolean } = {}
): { userId?: string; workspaceId?: string; tenantId?: string } {
  const userId =
    auth.callerContext?.user_id ??
    (auth.principal.token_use === 'user_access' || auth.principal.token_use === 'admin_access'
      ? auth.principal.sub
      : undefined);
  if (options.requireUserId && !userId) {
    throw new Error('caller_context_required');
  }
  return {
    userId,
    workspaceId: auth.callerContext?.workspace_id ?? auth.principal.workspace_id,
    tenantId: auth.callerContext?.tenant_id ?? auth.principal.tenant_id
  };
}
