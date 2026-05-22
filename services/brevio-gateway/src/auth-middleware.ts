import type http from 'node:http';

import {
  type AuthRuntimeConfig,
  type SessionTokenPayload,
  extractBearerToken,
  extractCookieToken,
  verifySessionToken
} from './auth.js';

export interface AuthenticatedContext {
  user_id: string;
  session_id: string;
  source: 'session' | 'dev_header';
}

export interface AuthFailure {
  status: 401 | 403;
  code: 'unauthenticated' | 'session_expired' | 'session_invalid';
  message: string;
}

function getHeader(req: http.IncomingMessage, name: string): string | undefined {
  const value = req.headers[name.toLowerCase()];
  if (typeof value === 'string') return value;
  if (Array.isArray(value) && value.length > 0) return value[0];
  return undefined;
}

export function authenticate(req: http.IncomingMessage, config: AuthRuntimeConfig): AuthenticatedContext | AuthFailure {
  const bearer = extractBearerToken(getHeader(req, 'authorization'));
  const cookie = extractCookieToken(getHeader(req, 'cookie'));
  const token = bearer ?? cookie;

  if (token) {
    const payload = verifySessionToken(config, token);
    if (!payload) {
      return { status: 401, code: 'session_invalid', message: 'session token invalid or expired' };
    }
    return { user_id: payload.user_id, session_id: payload.session_id, source: 'session' };
  }

  if (config.devMode) {
    const headerUser = getHeader(req, 'x-user-id');
    if (headerUser) {
      const headerSession = getHeader(req, 'x-session-id') ?? `dev-session-${headerUser}`;
      return { user_id: headerUser, session_id: headerSession, source: 'dev_header' };
    }
  }

  return { status: 401, code: 'unauthenticated', message: 'authentication required' };
}

export function isAuthFailure(value: AuthenticatedContext | AuthFailure): value is AuthFailure {
  return 'status' in value;
}

export function authPayloadFromVerified(payload: SessionTokenPayload): AuthenticatedContext {
  return { user_id: payload.user_id, session_id: payload.session_id, source: 'session' };
}
