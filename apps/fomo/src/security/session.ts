import { createHmac, timingSafeEqual } from 'node:crypto';

// Minimum-viable signed-session layer. NOT a full identity system — it only protects
// the future founder/admin endpoints once they go live (Phase 2B+).
//
// Token format: base64url(payloadJson) "." base64url(hmac)
//   payloadJson = { user_id, session_id, expires_at }
//
// Production refuses to boot without BREVIO_SESSION_SIGNING_KEY. Dev mode escape
// hatch: when BREVIO_DEV_MODE=true the session middleware accepts a plain x-user-id
// header instead of a signed token.

export interface SessionTokenPayload {
  user_id: string;
  session_id: string;
  expires_at: number;
}

export interface SessionRuntimeConfig {
  signingKey: Buffer | undefined;
  devMode: boolean;
}

export function loadSessionConfig(): SessionRuntimeConfig {
  const key = process.env.BREVIO_SESSION_SIGNING_KEY?.trim();
  const devMode = process.env.BREVIO_DEV_MODE === 'true';
  if (!key) {
    if (devMode) {
      return { signingKey: undefined, devMode: true };
    }
    throw new Error(
      'BREVIO_SESSION_SIGNING_KEY required in production. Set BREVIO_DEV_MODE=true to use the x-user-id header fallback (dev only).'
    );
  }
  const raw = key.startsWith('hex:') ? Buffer.from(key.slice(4), 'hex') : Buffer.from(key, 'base64');
  if (raw.length < 32) {
    throw new Error('BREVIO_SESSION_SIGNING_KEY must decode to at least 32 bytes');
  }
  return { signingKey: raw, devMode };
}

function base64url(buf: Buffer): string {
  return buf.toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function fromBase64url(text: string): Buffer {
  const pad = text.length % 4 === 0 ? '' : '='.repeat(4 - (text.length % 4));
  return Buffer.from(text.replace(/-/g, '+').replace(/_/g, '/') + pad, 'base64');
}

function computeHmac(signingKey: Buffer, payload: string): Buffer {
  return createHmac('sha256', signingKey).update(payload).digest();
}

export function signSessionToken(config: SessionRuntimeConfig, payload: SessionTokenPayload): string {
  if (!config.signingKey) {
    throw new Error('cannot sign session token without signing key');
  }
  const json = JSON.stringify(payload);
  const payloadEncoded = base64url(Buffer.from(json, 'utf8'));
  const sig = base64url(computeHmac(config.signingKey, payloadEncoded));
  return `${payloadEncoded}.${sig}`;
}

export function verifySessionToken(config: SessionRuntimeConfig, token: string | undefined): SessionTokenPayload | null {
  if (!token) return null;
  if (!config.signingKey) return null;
  const dot = token.indexOf('.');
  if (dot <= 0) return null;
  const payloadEncoded = token.slice(0, dot);
  const sigEncoded = token.slice(dot + 1);
  const expected = computeHmac(config.signingKey, payloadEncoded);
  let received: Buffer;
  try {
    received = fromBase64url(sigEncoded);
  } catch {
    return null;
  }
  if (received.length !== expected.length) return null;
  if (!timingSafeEqual(received, expected)) return null;
  let payload: SessionTokenPayload;
  try {
    payload = JSON.parse(fromBase64url(payloadEncoded).toString('utf8')) as SessionTokenPayload;
  } catch {
    return null;
  }
  if (!payload.user_id || !payload.session_id || typeof payload.expires_at !== 'number') return null;
  if (payload.expires_at < Math.floor(Date.now() / 1000)) return null;
  return payload;
}

export function extractBearerToken(authorizationHeader: string | undefined): string | undefined {
  if (!authorizationHeader) return undefined;
  const match = /^Bearer\s+(.+)$/i.exec(authorizationHeader.trim());
  return match?.[1];
}

export function extractCookieToken(cookieHeader: string | undefined, cookieName = 'brevio_session'): string | undefined {
  if (!cookieHeader) return undefined;
  for (const part of cookieHeader.split(';')) {
    const [name, ...valueParts] = part.trim().split('=');
    if (name === cookieName && valueParts.length > 0) {
      return valueParts.join('=');
    }
  }
  return undefined;
}
