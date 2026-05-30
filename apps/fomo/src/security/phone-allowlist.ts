// Phase v0.5.1 — Multi-tenant phone allowlist (storage + lookup).
//
// Founder corrections 2026-05-29 (see [[multitenant-design-principles]]):
//   * phone_e164_encrypted holds the KEK-wrapped ciphertext (jsonb
//     envelope shape: { v: key_version, ct: base64(nonce||ct||tag) }).
//     Decrypt ONLY at outbound-send time, never in log paths.
//   * phone_e164_hash holds a deterministic HMAC-SHA256 over the
//     normalized E.164, keyed by BREVIO_PHONE_HASH_KEY. This is the
//     lookup + uniqueness column — the encrypted column CANNOT be
//     deterministically indexed.
//   * Separation of duties: the HMAC key (BREVIO_PHONE_HASH_KEY) is
//     distinct from BREVIO_TOKEN_KEK. Rotating one does not invalidate
//     the other.
//   * NEVER log raw phone numbers. Audit detail uses the last-4
//     destination_slug pattern from 3E.1; this module exposes a
//     phoneSlug() helper.
//   * Duplicate-phone rejection fires at the DB UNIQUE constraint;
//     this module surfaces the violation as DuplicatePhoneError so
//     callers can branch cleanly. No application-layer "check before
//     insert" — that would race.

import { createHmac, randomBytes, timingSafeEqual } from 'node:crypto';
import { sql } from 'drizzle-orm';

import { type DrizzleClient } from '../db/client.js';
import { users } from '../db/schema.js';
import { type CryptoConfig, decryptToken, encryptToken } from './token-crypto.js';

const PHONE_HASH_KEY_BYTES = 32;

export interface PhoneHashConfig {
  readonly hmacKey: Buffer;
}

/**
 * Load the phone-hash HMAC config from env. Mirrors loadCryptoConfig
 * shape so the boot path can wire both with the same pattern.
 *
 * Requires BREVIO_PHONE_HASH_KEY to be a 32-byte key (base64 or
 * `hex:`-prefixed). Distinct from BREVIO_TOKEN_KEK by design — if
 * the founder ever rotates one, the other survives.
 *
 * Dev fallback (BREVIO_DEV_MODE=true) generates a per-process random
 * key; phone hashes computed in dev will not survive restart, which
 * is the correct behavior (we don't want a dev key to silently leak
 * into prod).
 */
export function loadPhoneHashConfig(env: NodeJS.ProcessEnv = process.env): PhoneHashConfig {
  const raw = env.BREVIO_PHONE_HASH_KEY?.trim();
  const devMode = env.BREVIO_DEV_MODE === 'true';
  if (!raw) {
    if (devMode) {
      // Dev-only path — per-process random key. Phone hashes won't
      // survive restart, which is desirable: dev keys must not leak.
      return { hmacKey: randomBytes(PHONE_HASH_KEY_BYTES) };
    }
    throw new Error(
      'BREVIO_PHONE_HASH_KEY required in production. ' +
        'Set BREVIO_DEV_MODE=true to use a per-process random key (dev only — phone hashes lost on restart).'
    );
  }
  const decoded = raw.startsWith('hex:')
    ? Buffer.from(raw.slice(4), 'hex')
    : Buffer.from(raw, 'base64');
  if (decoded.length !== PHONE_HASH_KEY_BYTES) {
    throw new Error(
      `BREVIO_PHONE_HASH_KEY must decode to exactly ${PHONE_HASH_KEY_BYTES} bytes; got ${decoded.length}`
    );
  }
  return { hmacKey: decoded };
}

/**
 * Strict E.164 normalization. Accepts input with optional surrounding
 * whitespace; requires `+` followed by 7-15 digits. Returns the
 * normalized form. Throws InvalidPhoneError on anything else.
 *
 * We intentionally do NOT try to "fix" loose inputs (no stripping
 * dashes, no country-code inference). The caller is responsible for
 * presenting clean E.164. Strictness keeps the hash deterministic
 * across surfaces.
 */
export function normalizeE164(input: string): string {
  if (typeof input !== 'string') {
    throw new InvalidPhoneError('phone must be a string');
  }
  const trimmed = input.trim();
  if (!/^\+\d{7,15}$/.test(trimmed)) {
    throw new InvalidPhoneError(
      `phone must be E.164 format (+ followed by 7-15 digits); got input of length ${trimmed.length}`
    );
  }
  return trimmed;
}

/**
 * Deterministic HMAC-SHA256 over the normalized E.164. Hex-encoded.
 *
 * Same plaintext + same key ALWAYS yields the same hash. Different
 * keys yield different hashes (HMAC, not just SHA). The hash IS the
 * primary lookup key — store it; index it; never log the plaintext.
 */
export function hashPhone(e164: string, cfg: PhoneHashConfig): string {
  const normalized = normalizeE164(e164);
  return createHmac('sha256', cfg.hmacKey).update(normalized, 'utf8').digest('hex');
}

/**
 * Constant-time comparison of two phone hashes. The hex digests are
 * the same length, so timingSafeEqual is safe. Defends against
 * theoretical timing attacks on hash equality at the routing layer.
 */
export function phoneHashesEqual(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  return timingSafeEqual(Buffer.from(a, 'utf8'), Buffer.from(b, 'utf8'));
}

/**
 * Last-4 slug for audit detail / log lines. Safe to include in
 * structured logs. Never reveal more than this 4-char suffix to
 * any observability surface; the full E.164 belongs only in the
 * encrypted column at rest + the outbound SendBlue API call.
 */
export function phoneSlug(e164: string): string {
  const normalized = normalizeE164(e164);
  return normalized.slice(-4);
}

/* ---------------------------------------------------------------------- */
/* Encrypted envelope storage shape                                       */
/* ---------------------------------------------------------------------- */

// What goes in the users.phone_e164_encrypted jsonb column. The
// envelope is self-describing: future KEK rotation can read v and
// dispatch to the right key.
export interface PhoneEnvelopeJson {
  readonly v: number;
  readonly ct: string;
}

const PHONE_AAD_PROVIDER = 'phone:e164';

export function encryptPhoneForUser(
  e164: string,
  user_id: string,
  crypto: CryptoConfig
): PhoneEnvelopeJson {
  const normalized = normalizeE164(e164);
  const { ciphertext, key_version } = encryptToken(crypto, normalized, user_id, PHONE_AAD_PROVIDER);
  return Object.freeze({
    v: key_version,
    ct: ciphertext.toString('base64')
  });
}

export function decryptPhoneForUser(
  envelope: PhoneEnvelopeJson,
  user_id: string,
  crypto: CryptoConfig
): string {
  if (!envelope || typeof envelope !== 'object' || typeof envelope.ct !== 'string') {
    throw new Error('phone envelope is malformed');
  }
  const ciphertext = Buffer.from(envelope.ct, 'base64');
  return decryptToken(crypto, ciphertext, envelope.v ?? 1, user_id, PHONE_AAD_PROVIDER);
}

// Phase v0.5.1 Step 4.1 — encrypt the friend's phone plaintext at
// invite-issue time, AAD-bound to the invite's intended_phone_hash
// so the ciphertext is per-invite (swapping ciphertexts across
// invites fails AEAD verification). Decrypted at /onboard/callback
// using the same AAD; the callback ALSO re-hashes the decrypted
// plaintext and verifies it equals the stored intended_phone_hash —
// any tamper at either column (encrypted or hash) fails closed.
const INVITE_AAD_PROVIDER = 'invite:phone';

export function encryptInviteBoundPhone(
  e164: string,
  intended_phone_hash: string,
  crypto: CryptoConfig
): PhoneEnvelopeJson {
  const normalized = normalizeE164(e164);
  const { ciphertext, key_version } = encryptToken(
    crypto,
    normalized,
    intended_phone_hash,
    INVITE_AAD_PROVIDER
  );
  return Object.freeze({
    v: key_version,
    ct: ciphertext.toString('base64')
  });
}

export function decryptInviteBoundPhone(
  envelope: PhoneEnvelopeJson,
  intended_phone_hash: string,
  crypto: CryptoConfig
): string {
  if (!envelope || typeof envelope !== 'object' || typeof envelope.ct !== 'string') {
    throw new Error('invite phone envelope is malformed');
  }
  const ciphertext = Buffer.from(envelope.ct, 'base64');
  return decryptToken(
    crypto,
    ciphertext,
    envelope.v ?? 1,
    intended_phone_hash,
    INVITE_AAD_PROVIDER
  );
}

/* ---------------------------------------------------------------------- */
/* Errors                                                                 */
/* ---------------------------------------------------------------------- */

export class InvalidPhoneError extends Error {
  constructor(reason: string) {
    super(`InvalidPhoneError: ${reason}`);
    this.name = 'InvalidPhoneError';
  }
}

export class DuplicatePhoneError extends Error {
  readonly existing_user_id: string | null;
  constructor(existing_user_id: string | null) {
    super(
      `DuplicatePhoneError: another user already owns this phone hash` +
        (existing_user_id ? ` (existing user_id slug=${existing_user_id.slice(0, 8)}…)` : '')
    );
    this.name = 'DuplicatePhoneError';
    this.existing_user_id = existing_user_id;
  }
}

/* ---------------------------------------------------------------------- */
/* Store interface + Postgres + in-memory implementations                 */
/* ---------------------------------------------------------------------- */

export interface PhoneAllowlistStore {
  // Provision a new friend's users row. Idempotent on (id) via
  // ON CONFLICT DO NOTHING — repeated callback hits (browser refresh,
  // OAuth retry) must not double-insert. Sets is_founder=false. The
  // caller MUST invoke this before setPhone — setPhone is a pure UPDATE
  // and silently affects 0 rows when the user_id doesn't exist yet.
  provisionUser(input: { user_id: string; email: string }): Promise<void>;
  // Set a user's phone. Idempotent for the same (user_id, hash) pair.
  // Throws DuplicatePhoneError when ANOTHER user already owns the hash.
  setPhone(user_id: string, e164: string): Promise<void>;
  // Inbound routing path. Returns null when no user owns this hash.
  findUserIdByPhoneHash(phone_hash: string): Promise<string | null>;
  // Outbound send path. Returns decrypted E.164 plaintext, or null
  // if the user has no phone on file. NEVER log the return value.
  getPhoneForUser(user_id: string): Promise<string | null>;
  // Convenience for tests / boot snapshot: is_founder lookup.
  isFounder(user_id: string): Promise<boolean>;
}

export class PostgresPhoneAllowlistStore implements PhoneAllowlistStore {
  private readonly db: DrizzleClient;
  private readonly crypto: CryptoConfig;
  private readonly phoneHash: PhoneHashConfig;

  constructor(db: DrizzleClient, crypto: CryptoConfig, phoneHash: PhoneHashConfig) {
    this.db = db;
    this.crypto = crypto;
    this.phoneHash = phoneHash;
  }

  async provisionUser(input: { user_id: string; email: string }): Promise<void> {
    // ON CONFLICT (id) DO NOTHING covers the "browser refreshed /onboard/callback
    // after success" case — the second hit must NOT throw. We also DO NOTHING
    // on email conflict for the same reason: if a prior run partially provisioned
    // (orphan rows after a crash), retrying with the same Gmail account
    // must be tolerated. The setPhone UNIQUE constraint is the real
    // identity guard, not this insert.
    await this.db.execute(
      sql`INSERT INTO ${users} (id, email, is_founder)
          VALUES (${input.user_id}::uuid, ${input.email}, false)
          ON CONFLICT (id) DO NOTHING`
    );
  }

  async setPhone(user_id: string, e164: string): Promise<void> {
    const normalized = normalizeE164(e164);
    const hash = hashPhone(normalized, this.phoneHash);
    const envelope = encryptPhoneForUser(normalized, user_id, this.crypto);

    // Insert-or-update with idempotency: if the SAME user_id already
    // owns this hash, this is a no-op. If a DIFFERENT user_id owns
    // the hash, the UNIQUE constraint fires and we surface
    // DuplicatePhoneError. The application NEVER does a "check before
    // insert" — that races.
    try {
      await this.db.execute(
        sql`UPDATE ${users}
            SET phone_e164_encrypted = ${envelope as unknown as object},
                phone_e164_hash = ${hash}
            WHERE id::text = ${user_id}`
      );
      // If the UPDATE matched 0 rows, the user_id doesn't exist; the
      // caller should provision the users row via the /onboard flow
      // first. We don't auto-create here — explicit is safer.
    } catch (err) {
      // Map UNIQUE violation to DuplicatePhoneError.
      const msg = err instanceof Error ? err.message : String(err);
      if (/users_phone_e164_hash_uq|duplicate key value violates unique constraint/i.test(msg)) {
        const existing = await this.findUserIdByPhoneHash(hash);
        throw new DuplicatePhoneError(existing);
      }
      throw err;
    }
  }

  async findUserIdByPhoneHash(phone_hash: string): Promise<string | null> {
    const rows = await this.db
      .select({ id: users.id })
      .from(users)
      .where(sql`${users.phone_e164_hash} = ${phone_hash}`)
      .limit(1);
    const row = rows[0];
    return row ? (row.id as unknown as string) : null;
  }

  async getPhoneForUser(user_id: string): Promise<string | null> {
    const rows = await this.db
      .select({ envelope: users.phone_e164_encrypted })
      .from(users)
      .where(sql`${users.id}::text = ${user_id}`)
      .limit(1);
    const row = rows[0];
    if (!row || !row.envelope) return null;
    return decryptPhoneForUser(row.envelope as unknown as PhoneEnvelopeJson, user_id, this.crypto);
  }

  async isFounder(user_id: string): Promise<boolean> {
    const rows = await this.db
      .select({ is_founder: users.is_founder })
      .from(users)
      .where(sql`${users.id}::text = ${user_id}`)
      .limit(1);
    return rows[0]?.is_founder === true;
  }
}

// In-memory backed by a simple Map keyed by user_id. Used by unit
// tests. Behavior mirrors the Postgres store for the surface area we
// test against; duplicate-phone rejection is enforced application-side
// here (no SQL UNIQUE) so the failure mode is testable without PGlite.
export class InMemoryPhoneAllowlistStore implements PhoneAllowlistStore {
  private readonly byUser = new Map<string, { hash: string; envelope: PhoneEnvelopeJson; is_founder: boolean; email?: string }>();
  private readonly crypto: CryptoConfig;
  private readonly phoneHash: PhoneHashConfig;

  constructor(crypto: CryptoConfig, phoneHash: PhoneHashConfig) {
    this.crypto = crypto;
    this.phoneHash = phoneHash;
  }

  async provisionUser(input: { user_id: string; email: string }): Promise<void> {
    // Idempotent: if user already exists, leave as-is.
    if (this.byUser.has(input.user_id)) return;
    // Placeholder row — setPhone will fill the hash + envelope. The
    // placeholder hash is empty so it can't collide with any real
    // hash via findUserIdByPhoneHash.
    this.byUser.set(input.user_id, {
      hash: '',
      envelope: { v: 0, alg: 'placeholder', ciphertext: '', iv: '', tag: '' } as unknown as PhoneEnvelopeJson,
      is_founder: false,
      email: input.email
    });
  }

  async setPhone(user_id: string, e164: string): Promise<void> {
    const normalized = normalizeE164(e164);
    const hash = hashPhone(normalized, this.phoneHash);

    // Duplicate-phone check: another user owns this hash?
    for (const [otherUid, row] of this.byUser.entries()) {
      if (otherUid !== user_id && row.hash === hash) {
        throw new DuplicatePhoneError(otherUid);
      }
    }

    const envelope = encryptPhoneForUser(normalized, user_id, this.crypto);
    const existing = this.byUser.get(user_id);
    this.byUser.set(user_id, {
      hash,
      envelope,
      is_founder: existing?.is_founder ?? false
    });
  }

  async findUserIdByPhoneHash(phone_hash: string): Promise<string | null> {
    // Empty phone_hash never matches — protects against placeholder
    // rows from provisionUser that haven't had setPhone called yet.
    if (phone_hash === '') return null;
    for (const [uid, row] of this.byUser.entries()) {
      if (row.hash === phone_hash) return uid;
    }
    return null;
  }

  async getPhoneForUser(user_id: string): Promise<string | null> {
    const row = this.byUser.get(user_id);
    if (!row) return null;
    return decryptPhoneForUser(row.envelope, user_id, this.crypto);
  }

  async isFounder(user_id: string): Promise<boolean> {
    return this.byUser.get(user_id)?.is_founder === true;
  }

  // Test-only seam to mark a user as the founder. Production calls
  // this via the /onboard provisioning path (founder is bootstrapped
  // at first boot with is_founder=true).
  markFounder(user_id: string): void {
    const existing = this.byUser.get(user_id);
    if (existing) {
      this.byUser.set(user_id, { ...existing, is_founder: true });
    } else {
      this.byUser.set(user_id, {
        hash: '',
        envelope: { v: 1, ct: '' },
        is_founder: true
      });
    }
  }
}
