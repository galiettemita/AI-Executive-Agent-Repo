// Phase v0.5.1 — Invite token store for the /onboard friend-beta flow.
//
// Founder corrections 2026-05-29 ([[multitenant-design-principles]] §5):
//
//   * Store ONLY the sha256 hash of the token. The plaintext is
//     returned to the founder ONCE at issue time and NEVER persisted.
//   * NEVER log the plaintext token. Audit detail uses an 8-char
//     hash prefix (`token_hash_prefix`) — the same redaction
//     discipline used for SENDBLUE_WEBHOOK_SECRET and BREVIO_TOKEN_KEK.
//   * Consume AFTER successful OAuth callback / user creation, not
//     on the initial /onboard GET. The store's `consume()` performs
//     the atomic conditional UPDATE that proves the token was valid
//     at the moment of consumption.
//   * Token must be one-time + expiring. The atomic UPDATE's WHERE
//     clause (`consumed_at IS NULL AND expires_at > now()`) is the
//     single source of truth for "this token is still valid."
//   * Token binds to intended phone (`intended_phone_hash`). The
//     /onboard callback must verify the friend's resolved phone matches.

import { createHash, randomBytes, timingSafeEqual } from 'node:crypto';
import { sql } from 'drizzle-orm';

import { type DrizzleClient } from '../db/client.js';
import { invite_tokens } from '../db/schema.js';
import { type PhoneEnvelopeJson } from './phone-allowlist.js';

const TOKEN_PLAINTEXT_BYTES = 32; // 256 bits of entropy → 43-char base64url

// 24h default TTL — short enough that a leaked invite URL is mostly
// useless by next day; long enough that a friend doesn't have to
// onboard immediately.
const DEFAULT_TTL_MS = 24 * 60 * 60 * 1000;

/* ---------------------------------------------------------------------- */
/* Token generation + hashing                                             */
/* ---------------------------------------------------------------------- */

/**
 * Generate a fresh random invite-token plaintext. 32 bytes (256 bits)
 * of randomness encoded URL-safe base64 (no padding). 43 chars.
 *
 * This value is returned to the founder ONCE, written into the
 * /onboard URL, sent to the friend, and never again touched by the
 * runtime. The DB stores only the hash.
 */
export function generateTokenPlaintext(): string {
  const bytes = randomBytes(TOKEN_PLAINTEXT_BYTES);
  return base64url(bytes);
}

/**
 * Deterministic sha256 hex of the token plaintext. Used as the DB
 * lookup key + UNIQUE index target.
 *
 * Unlike phone hashing, this is unkeyed sha256 (not HMAC). The token
 * IS the secret — there is no separate key. An attacker with the
 * hash cannot reverse to the plaintext (assuming the plaintext is
 * high-entropy, which generateTokenPlaintext guarantees).
 */
export function hashToken(plaintext: string): string {
  return createHash('sha256').update(plaintext, 'utf8').digest('hex');
}

/**
 * Constant-time comparison of two token hashes. Defends against
 * theoretical timing attacks at the routing layer.
 */
export function tokenHashesEqual(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  return timingSafeEqual(Buffer.from(a, 'utf8'), Buffer.from(b, 'utf8'));
}

/**
 * 8-char prefix of the hash, safe to include in audit detail / log
 * lines for traceability. Even with the prefix leaked, the full hash
 * is not reversible to the plaintext.
 */
export function tokenHashPrefix(hash: string): string {
  return hash.slice(0, 8);
}

function base64url(bytes: Buffer): string {
  return bytes
    .toString('base64')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
}

/* ---------------------------------------------------------------------- */
/* Types                                                                  */
/* ---------------------------------------------------------------------- */

export interface IssueInviteInput {
  readonly intended_phone_hash: string;
  // Phase v0.5.1 Step 4.1 — KEK-wrapped phone envelope bound to the
  // invite's intended_phone_hash via AAD. The caller (issue script)
  // computes this via encryptInviteBoundPhone() from phone-allowlist.
  // REQUIRED on every issue — the store rejects undefined/null.
  readonly intended_phone_encrypted: PhoneEnvelopeJson;
  readonly issued_by_user_id: string;
  // Optional override; defaults to 24h.
  readonly ttl_ms?: number;
}

export interface IssueInviteResult {
  // The PLAINTEXT token. Returned to the founder for sending to the
  // friend, then never accessible again. The runtime DOES NOT persist
  // this anywhere.
  readonly token_plaintext: string;
  // The DB row id of the issued token, for audit correlation.
  readonly id: number;
  // The hash that was actually stored. Useful for the audit row
  // (the audit detail surfaces tokenHashPrefix(token_hash)).
  readonly token_hash: string;
  // The expiry timestamp, for the founder to show the friend.
  readonly expires_at: Date;
}

export interface InviteTokenRecord {
  readonly id: number;
  readonly token_hash: string;
  readonly intended_phone_hash: string;
  readonly intended_phone_encrypted: PhoneEnvelopeJson | null;
  readonly issued_by_user_id: string;
  readonly issued_at: Date;
  readonly expires_at: Date;
  readonly consumed_at: Date | null;
  readonly consumed_user_id: string | null;
}

export interface ConsumeInput {
  readonly token_hash: string;
  readonly consumed_user_id: string;
  // Injected `now` for tests; defaults to new Date().
  readonly now?: Date;
}

export interface ConsumeResult {
  readonly id: number;
  readonly intended_phone_hash: string;
  // Phase v0.5.1 Step 4.1 — caller decrypts with the same hash as AAD
  // and verifies hashPhone(decrypted) === intended_phone_hash before
  // using the plaintext. Tampered ciphertext fails the AEAD check;
  // tampered hash column fails the post-decrypt round-trip check.
  readonly intended_phone_encrypted: PhoneEnvelopeJson | null;
}

export class InvalidInviteTokenError extends Error {
  readonly reason: 'unknown' | 'expired' | 'consumed';
  constructor(reason: 'unknown' | 'expired' | 'consumed') {
    super(`InvalidInviteTokenError: ${reason}`);
    this.name = 'InvalidInviteTokenError';
    this.reason = reason;
  }
}

/* ---------------------------------------------------------------------- */
/* Store interface                                                        */
/* ---------------------------------------------------------------------- */

export interface InviteTokenStore {
  // Issues a fresh token. Generates plaintext + stores ONLY the hash.
  issue(input: IssueInviteInput, now?: Date): Promise<IssueInviteResult>;
  // Look up a token row by hash. Used by /onboard GET to check
  // validity BEFORE rendering the consent page. Returns null when
  // the token doesn't exist. NOTE: looking up a valid token does
  // NOT consume it — consumption is a separate atomic call.
  lookupByHash(token_hash: string): Promise<InviteTokenRecord | null>;
  // Atomic single-call consume. Returns the consumed row's
  // identity binding (id + intended_phone_hash) on success. Throws
  // InvalidInviteTokenError on any failure mode (unknown, expired,
  // already consumed). Idempotent: a second call with the same
  // arguments throws InvalidInviteTokenError('consumed').
  consume(input: ConsumeInput): Promise<ConsumeResult>;
}

/* ---------------------------------------------------------------------- */
/* Postgres implementation                                                */
/* ---------------------------------------------------------------------- */

export class PostgresInviteTokenStore implements InviteTokenStore {
  private readonly db: DrizzleClient;

  constructor(db: DrizzleClient) {
    this.db = db;
  }

  async issue(input: IssueInviteInput, now: Date = new Date()): Promise<IssueInviteResult> {
    assertEncryptedEnvelope(input.intended_phone_encrypted);
    const token_plaintext = generateTokenPlaintext();
    const token_hash = hashToken(token_plaintext);
    const ttl = input.ttl_ms ?? DEFAULT_TTL_MS;
    const expires_at = new Date(now.getTime() + ttl);

    const rows = await this.db
      .insert(invite_tokens)
      .values({
        token_hash,
        intended_phone_hash: input.intended_phone_hash,
        intended_phone_encrypted: input.intended_phone_encrypted as unknown as object,
        issued_by_user_id: input.issued_by_user_id,
        issued_at: now,
        expires_at,
        consumed_at: null,
        consumed_user_id: null
      })
      .returning({ id: invite_tokens.id });
    const row = rows[0];
    if (!row) {
      throw new Error('invite-tokens: insert returned no row');
    }
    return Object.freeze({
      token_plaintext,
      id: row.id,
      token_hash,
      expires_at
    });
  }

  async lookupByHash(token_hash: string): Promise<InviteTokenRecord | null> {
    const rows = await this.db
      .select()
      .from(invite_tokens)
      .where(sql`${invite_tokens.token_hash} = ${token_hash}`)
      .limit(1);
    const row = rows[0];
    if (!row) return null;
    return freezeRecord(row);
  }

  async consume(input: ConsumeInput): Promise<ConsumeResult> {
    const now = input.now ?? new Date();
    // Atomic conditional UPDATE — the WHERE clause is the single
    // source of truth for "this token is valid right now."
    const rows = await this.db.execute(
      sql`UPDATE invite_tokens
          SET consumed_at = ${now}, consumed_user_id = ${input.consumed_user_id}
          WHERE token_hash = ${input.token_hash}
            AND consumed_at IS NULL
            AND expires_at > ${now}
          RETURNING id, intended_phone_hash, intended_phone_encrypted`
    );
    const r = rows.rows[0] as
      | { id: number; intended_phone_hash: string; intended_phone_encrypted: PhoneEnvelopeJson | null }
      | undefined;
    if (r) {
      return Object.freeze({
        id: r.id,
        intended_phone_hash: r.intended_phone_hash,
        intended_phone_encrypted: r.intended_phone_encrypted
      });
    }
    // Disambiguate WHY the consume failed so the caller (/onboard
    // callback) can emit the right audit reason. Cheap lookup to
    // distinguish "unknown / expired / already consumed."
    const existing = await this.lookupByHash(input.token_hash);
    if (!existing) throw new InvalidInviteTokenError('unknown');
    if (existing.consumed_at !== null) throw new InvalidInviteTokenError('consumed');
    throw new InvalidInviteTokenError('expired');
  }
}

function freezeRecord(row: Record<string, unknown>): InviteTokenRecord {
  return Object.freeze({
    id: row.id as number,
    token_hash: row.token_hash as string,
    intended_phone_hash: row.intended_phone_hash as string,
    intended_phone_encrypted: (row.intended_phone_encrypted as PhoneEnvelopeJson | null) ?? null,
    issued_by_user_id: row.issued_by_user_id as string,
    issued_at: row.issued_at as Date,
    expires_at: row.expires_at as Date,
    consumed_at: (row.consumed_at as Date | null) ?? null,
    consumed_user_id: (row.consumed_user_id as string | null) ?? null
  });
}

// Fail-loud guard. The runtime path requires a valid envelope on
// every new issue; nullable column exists only for back-compat with
// pre-step-4.1 test fixtures.
function assertEncryptedEnvelope(env: PhoneEnvelopeJson | undefined | null): asserts env is PhoneEnvelopeJson {
  if (!env || typeof env !== 'object' || typeof env.ct !== 'string' || env.ct.length === 0) {
    throw new Error(
      'invite-tokens: intended_phone_encrypted envelope is required (Step 4.1 — no half-wired issues).'
    );
  }
}

/* ---------------------------------------------------------------------- */
/* In-memory implementation (tests only)                                  */
/* ---------------------------------------------------------------------- */

export class InMemoryInviteTokenStore implements InviteTokenStore {
  private readonly rows = new Map<string, {
    id: number;
    token_hash: string;
    intended_phone_hash: string;
    intended_phone_encrypted: PhoneEnvelopeJson | null;
    issued_by_user_id: string;
    issued_at: Date;
    expires_at: Date;
    consumed_at: Date | null;
    consumed_user_id: string | null;
  }>();
  private nextId = 1;

  async issue(input: IssueInviteInput, now: Date = new Date()): Promise<IssueInviteResult> {
    assertEncryptedEnvelope(input.intended_phone_encrypted);
    const token_plaintext = generateTokenPlaintext();
    const token_hash = hashToken(token_plaintext);
    if (this.rows.has(token_hash)) {
      // Astronomically unlikely (32-byte random) but defends against
      // a tiny test fixture seeding the same hash twice.
      throw new Error('invite-tokens: hash collision on issue');
    }
    const ttl = input.ttl_ms ?? DEFAULT_TTL_MS;
    const expires_at = new Date(now.getTime() + ttl);
    const id = this.nextId++;
    this.rows.set(token_hash, {
      id,
      token_hash,
      intended_phone_hash: input.intended_phone_hash,
      intended_phone_encrypted: input.intended_phone_encrypted,
      issued_by_user_id: input.issued_by_user_id,
      issued_at: now,
      expires_at,
      consumed_at: null,
      consumed_user_id: null
    });
    return Object.freeze({ token_plaintext, id, token_hash, expires_at });
  }

  async lookupByHash(token_hash: string): Promise<InviteTokenRecord | null> {
    const row = this.rows.get(token_hash);
    if (!row) return null;
    return freezeRecord(row);
  }

  async consume(input: ConsumeInput): Promise<ConsumeResult> {
    const now = input.now ?? new Date();
    const row = this.rows.get(input.token_hash);
    if (!row) throw new InvalidInviteTokenError('unknown');
    if (row.consumed_at !== null) throw new InvalidInviteTokenError('consumed');
    if (row.expires_at.getTime() <= now.getTime()) throw new InvalidInviteTokenError('expired');
    // Mutate in place — the conditional check above is the atomic
    // analog for in-memory; in Postgres the conditional UPDATE WHERE
    // clause is the source of truth.
    row.consumed_at = now;
    row.consumed_user_id = input.consumed_user_id;
    return Object.freeze({
      id: row.id,
      intended_phone_hash: row.intended_phone_hash,
      intended_phone_encrypted: row.intended_phone_encrypted
    });
  }
}
