// OAuth state HMAC + PKCE code verifier helpers, plus an in-memory nonce store.
// State is bound to the user_id to prevent OAuth fixation.

import { createHash, createHmac, randomBytes, timingSafeEqual } from 'node:crypto';

const NONCE_BYTES = 16;
const PKCE_VERIFIER_BYTES = 32;
const STATE_TTL_MS = 10 * 60 * 1000;

export interface OAuthStateConfig {
  signingKey: Buffer;
}

export function loadOAuthStateConfig(): OAuthStateConfig {
  const raw = process.env.BREVIO_OAUTH_STATE_KEY?.trim();
  const devMode = process.env.BREVIO_DEV_MODE === 'true';
  if (!raw) {
    if (devMode) {
      return { signingKey: randomBytes(32) };
    }
    throw new Error('BREVIO_OAUTH_STATE_KEY required (or BREVIO_DEV_MODE=true for a per-process random key in dev)');
  }
  const decoded = raw.startsWith('hex:')
    ? Buffer.from(raw.slice(4), 'hex')
    : Buffer.from(raw, 'base64');
  if (decoded.length < 32) {
    throw new Error('BREVIO_OAUTH_STATE_KEY must decode to at least 32 bytes');
  }
  return { signingKey: decoded };
}

export interface StateClaims {
  user_id: string;
  provider: string;
  skill_id: string;
  pending_message_id: string | null;
  iat: number;
  nonce: string;
}

function base64url(buf: Buffer): string {
  return buf.toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function fromBase64url(text: string): Buffer {
  const pad = text.length % 4 === 0 ? '' : '='.repeat(4 - (text.length % 4));
  return Buffer.from(text.replace(/-/g, '+').replace(/_/g, '/') + pad, 'base64');
}

export function generatePKCEVerifier(): string {
  return base64url(randomBytes(PKCE_VERIFIER_BYTES));
}

export function deriveCodeChallenge(verifier: string): string {
  return base64url(createHash('sha256').update(verifier).digest());
}

export function generateNonce(): string {
  return base64url(randomBytes(NONCE_BYTES));
}

export function buildState(config: OAuthStateConfig, claims: StateClaims): string {
  const payload = base64url(Buffer.from(JSON.stringify(claims), 'utf8'));
  const sig = base64url(createHmac('sha256', config.signingKey).update(payload).digest());
  return `${payload}.${sig}`;
}

export function verifyState(config: OAuthStateConfig, state: string, now: number = Date.now()): StateClaims | null {
  const dot = state.indexOf('.');
  if (dot <= 0) return null;
  const payload = state.slice(0, dot);
  const sigEncoded = state.slice(dot + 1);
  const expected = createHmac('sha256', config.signingKey).update(payload).digest();
  let received: Buffer;
  try {
    received = fromBase64url(sigEncoded);
  } catch {
    return null;
  }
  if (received.length !== expected.length) return null;
  if (!timingSafeEqual(received, expected)) return null;
  let claims: StateClaims;
  try {
    claims = JSON.parse(fromBase64url(payload).toString('utf8')) as StateClaims;
  } catch {
    return null;
  }
  if (typeof claims.iat !== 'number') return null;
  if (now - claims.iat > STATE_TTL_MS) return null;
  if (now < claims.iat - 60_000) return null;
  return claims;
}

export interface NonceRow {
  nonce: string;
  user_id: string;
  provider: string;
  skill_id: string;
  code_verifier: string;
  pending_message_id: string | null;
  created_at: number;
  consumed: boolean;
}

export interface NonceStore {
  put(row: NonceRow): Promise<void>;
  consume(nonce: string): Promise<NonceRow | null>;
  prune(now?: number): Promise<number>;
}

export class InMemoryNonceStore implements NonceStore {
  private readonly rows = new Map<string, NonceRow>();
  async put(row: NonceRow): Promise<void> {
    this.rows.set(row.nonce, row);
  }
  async consume(nonce: string): Promise<NonceRow | null> {
    const row = this.rows.get(nonce);
    if (!row) return null;
    if (row.consumed) return null;
    row.consumed = true;
    return row;
  }
  async prune(now: number = Date.now()): Promise<number> {
    let pruned = 0;
    for (const [k, v] of this.rows) {
      if (now - v.created_at > STATE_TTL_MS) {
        this.rows.delete(k);
        pruned++;
      }
    }
    return pruned;
  }
}

export const STATE_TTL_MS_EXPORT = STATE_TTL_MS;
