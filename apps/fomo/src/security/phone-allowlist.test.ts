// Phase v0.5.1 step #2 — regression tests for the phone allowlist.
//
// Founder corrections covered:
//   #1 Phone lookup: HMAC over normalized E.164; encrypted column
//      separately; never index the encrypted column directly.
//   #2 Don't log raw phone numbers (test via toString / Error.message
//      shape — phones never appear).
//   #3 Duplicate-phone rejection via DuplicatePhoneError; first row
//      untouched.
//   #4 Synthetic test phones are DISTINCT (no shared founder phone
//      in fixtures).
//
// Note on synthetic phone numbers: we use distinct numbers in the
// US +1 555-01xx reserved-for-fiction range (per founder directive
// 2026-05-29 — synthetic test data must be distinct from production).

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { type CryptoConfig } from './token-crypto.js';
import {
  DuplicatePhoneError,
  InMemoryPhoneAllowlistStore,
  InvalidPhoneError,
  decryptPhoneForUser,
  encryptPhoneForUser,
  hashPhone,
  loadPhoneHashConfig,
  normalizeE164,
  phoneHashesEqual,
  phoneSlug,
  type PhoneHashConfig
} from './phone-allowlist.js';

// Distinct synthetic phones — never reuse the founder's real number
// in tests (corrections #4). +1-555-01xx is reserved-for-fiction.
const FOUNDER_PHONE = '+15550100001';
const FRIEND_PHONE = '+15550100002';
const FRIEND_PHONE_2 = '+15550100003';

const TEST_KEK = Buffer.alloc(32, 13).toString('base64');
const TEST_HASH_KEY = Buffer.alloc(32, 91).toString('base64');

function withEnv<T>(overrides: Record<string, string | undefined>, fn: () => T): T {
  const prev: Record<string, string | undefined> = {};
  for (const [k, v] of Object.entries(overrides)) {
    prev[k] = process.env[k];
    if (v === undefined) delete process.env[k];
    else process.env[k] = v;
  }
  try {
    return fn();
  } finally {
    for (const [k, v] of Object.entries(prev)) {
      if (v === undefined) delete process.env[k];
      else process.env[k] = v;
    }
  }
}

const cryptoConfig: CryptoConfig = {
  kek: Buffer.from(TEST_KEK, 'base64'),
  devMode: false
};
const phoneHashConfig: PhoneHashConfig = {
  hmacKey: Buffer.from(TEST_HASH_KEY, 'base64')
};

/* ---------------------------------------------------------------------- */
/* normalizeE164 — strict input shape                                     */
/* ---------------------------------------------------------------------- */

describe('normalizeE164', () => {
  it('accepts a clean E.164 string and returns it unchanged', () => {
    assert.equal(normalizeE164(FOUNDER_PHONE), FOUNDER_PHONE);
  });

  it('trims surrounding whitespace but rejects internal whitespace', () => {
    assert.equal(normalizeE164(`  ${FOUNDER_PHONE}  `), FOUNDER_PHONE);
    assert.throws(() => normalizeE164('+1 555 0100001'), InvalidPhoneError);
  });

  it('rejects missing leading + and other malformed inputs', () => {
    for (const bad of ['15550100001', '+abc', '+', '+1234', '+12345678901234567', '']) {
      assert.throws(() => normalizeE164(bad), InvalidPhoneError, `expected reject of '${bad}'`);
    }
  });

  it('InvalidPhoneError message does NOT include the raw input value (privacy invariant)', () => {
    try {
      normalizeE164('+abc-CANARY-MUST-NOT-LEAK');
      assert.fail('should have thrown');
    } catch (err) {
      assert.ok(err instanceof InvalidPhoneError);
      assert.equal(
        err.message.includes('CANARY-MUST-NOT-LEAK'),
        false,
        'InvalidPhoneError message must NOT include the raw input'
      );
    }
  });

  it('rejects non-string input cleanly', () => {
    // @ts-expect-error — intentional misuse for runtime check
    assert.throws(() => normalizeE164(null), InvalidPhoneError);
    // @ts-expect-error — intentional misuse for runtime check
    assert.throws(() => normalizeE164(123), InvalidPhoneError);
  });
});

/* ---------------------------------------------------------------------- */
/* hashPhone — HMAC determinism + key-dependence                          */
/* ---------------------------------------------------------------------- */

describe('hashPhone (HMAC-SHA256)', () => {
  it('is deterministic — same phone + same key always yields same hash', () => {
    const h1 = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    const h2 = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    assert.equal(h1, h2);
  });

  it('produces 64-char hex digests (SHA-256 output length)', () => {
    const h = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    assert.equal(h.length, 64);
    assert.match(h, /^[0-9a-f]{64}$/);
  });

  it('different phones with the SAME key yield different hashes', () => {
    const h1 = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    const h2 = hashPhone(FRIEND_PHONE, phoneHashConfig);
    assert.notEqual(h1, h2);
  });

  it('same phone with DIFFERENT keys yields different hashes (key-dependence — separation of duties)', () => {
    const altKey: PhoneHashConfig = { hmacKey: Buffer.alloc(32, 250) };
    const h1 = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    const h2 = hashPhone(FOUNDER_PHONE, altKey);
    assert.notEqual(h1, h2);
  });

  it('rejects inputs that fail normalization (e.g. whitespace within the number)', () => {
    assert.throws(() => hashPhone('+1 555 0100001', phoneHashConfig), InvalidPhoneError);
  });

  it('hash output does NOT include the raw phone (string-search check)', () => {
    const h = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    assert.equal(h.includes('5550100001'), false);
  });
});

/* ---------------------------------------------------------------------- */
/* phoneHashesEqual — constant-time comparison                            */
/* ---------------------------------------------------------------------- */

describe('phoneHashesEqual', () => {
  it('returns true for equal hashes', () => {
    const h = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    assert.equal(phoneHashesEqual(h, h), true);
  });

  it('returns false for different hashes', () => {
    const h1 = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    const h2 = hashPhone(FRIEND_PHONE, phoneHashConfig);
    assert.equal(phoneHashesEqual(h1, h2), false);
  });

  it('returns false for hashes of different lengths (defense-in-depth)', () => {
    assert.equal(phoneHashesEqual('a'.repeat(64), 'a'.repeat(63)), false);
  });
});

/* ---------------------------------------------------------------------- */
/* phoneSlug — last-4 audit-safe identifier                               */
/* ---------------------------------------------------------------------- */

describe('phoneSlug', () => {
  it('returns last 4 chars of normalized E.164', () => {
    assert.equal(phoneSlug(FOUNDER_PHONE), '0001');
    assert.equal(phoneSlug(FRIEND_PHONE), '0002');
  });

  it('normalizes the input before slicing', () => {
    assert.equal(phoneSlug(`  ${FOUNDER_PHONE}  `), '0001');
  });

  it('rejects malformed inputs (does NOT silently slice garbage)', () => {
    assert.throws(() => phoneSlug('abc'), InvalidPhoneError);
  });
});

/* ---------------------------------------------------------------------- */
/* encrypt/decrypt round-trip                                             */
/* ---------------------------------------------------------------------- */

describe('encryptPhoneForUser / decryptPhoneForUser', () => {
  it('round-trips a phone for the same user_id', () => {
    const userId = 'user-fixture-1';
    const envelope = encryptPhoneForUser(FOUNDER_PHONE, userId, cryptoConfig);
    const decrypted = decryptPhoneForUser(envelope, userId, cryptoConfig);
    assert.equal(decrypted, FOUNDER_PHONE);
  });

  it('envelope contains a key_version + base64 ciphertext (the on-disk shape)', () => {
    const envelope = encryptPhoneForUser(FOUNDER_PHONE, 'user-fixture', cryptoConfig);
    assert.equal(typeof envelope.v, 'number');
    assert.equal(typeof envelope.ct, 'string');
    assert.ok(envelope.ct.length > 0);
    // Plaintext must NEVER appear in the ciphertext base64.
    const decoded = Buffer.from(envelope.ct, 'base64').toString('binary');
    assert.equal(decoded.includes('5550100001'), false);
  });

  it('decrypt with a DIFFERENT user_id fails (AAD binding)', () => {
    const envelope = encryptPhoneForUser(FOUNDER_PHONE, 'user-A', cryptoConfig);
    assert.throws(() => decryptPhoneForUser(envelope, 'user-B', cryptoConfig));
  });
});

/* ---------------------------------------------------------------------- */
/* InMemoryPhoneAllowlistStore — setPhone / findUserIdByPhoneHash         */
/* ---------------------------------------------------------------------- */

describe('InMemoryPhoneAllowlistStore', () => {
  function makeStore(): InMemoryPhoneAllowlistStore {
    return new InMemoryPhoneAllowlistStore(cryptoConfig, phoneHashConfig);
  }

  it('setPhone + findUserIdByPhoneHash round-trips for one user', async () => {
    const store = makeStore();
    await store.setPhone('founder', FOUNDER_PHONE);
    const hash = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    const uid = await store.findUserIdByPhoneHash(hash);
    assert.equal(uid, 'founder');
  });

  it('getPhoneForUser decrypts the stored E.164', async () => {
    const store = makeStore();
    await store.setPhone('founder', FOUNDER_PHONE);
    const plain = await store.getPhoneForUser('founder');
    assert.equal(plain, FOUNDER_PHONE);
  });

  it('findUserIdByPhoneHash returns null for an unknown hash', async () => {
    const store = makeStore();
    const uid = await store.findUserIdByPhoneHash('a'.repeat(64));
    assert.equal(uid, null);
  });

  it('getPhoneForUser returns null for an unknown user_id', async () => {
    const store = makeStore();
    const plain = await store.getPhoneForUser('nobody');
    assert.equal(plain, null);
  });

  it('setPhone is idempotent for the same (user_id, phone) pair', async () => {
    const store = makeStore();
    await store.setPhone('founder', FOUNDER_PHONE);
    await store.setPhone('founder', FOUNDER_PHONE);
    const hash = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    assert.equal(await store.findUserIdByPhoneHash(hash), 'founder');
  });

  it('two distinct users with distinct phones both resolve correctly (multi-tenant invariant)', async () => {
    const store = makeStore();
    await store.setPhone('founder', FOUNDER_PHONE);
    await store.setPhone('friend-1', FRIEND_PHONE);

    assert.equal(
      await store.findUserIdByPhoneHash(hashPhone(FOUNDER_PHONE, phoneHashConfig)),
      'founder'
    );
    assert.equal(
      await store.findUserIdByPhoneHash(hashPhone(FRIEND_PHONE, phoneHashConfig)),
      'friend-1'
    );
    // Cross-resolution must NOT happen — founder hash never resolves to friend.
    assert.notEqual(
      await store.findUserIdByPhoneHash(hashPhone(FRIEND_PHONE, phoneHashConfig)),
      'founder'
    );
  });

  it('DuplicatePhoneError fires when a second user tries to claim a phone already owned (correction #3)', async () => {
    const store = makeStore();
    await store.setPhone('founder', FOUNDER_PHONE);

    let thrown: unknown;
    try {
      await store.setPhone('friend-imposter', FOUNDER_PHONE);
    } catch (err) {
      thrown = err;
    }
    assert.ok(thrown instanceof DuplicatePhoneError, 'expected DuplicatePhoneError');
    assert.equal((thrown as DuplicatePhoneError).existing_user_id, 'founder');

    // First row UNTOUCHED — duplicate rejection must not corrupt
    // the existing user's allowlist row.
    const hash = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    assert.equal(await store.findUserIdByPhoneHash(hash), 'founder');
    assert.equal(await store.getPhoneForUser('founder'), FOUNDER_PHONE);
    assert.equal(await store.getPhoneForUser('friend-imposter'), null);
  });

  it('DuplicatePhoneError message uses a slug — does NOT include the existing user_id in full / phone plaintext', async () => {
    const store = makeStore();
    await store.setPhone('founder-uuid-long-12345678', FOUNDER_PHONE);
    try {
      await store.setPhone('friend-uuid-different-87654321', FOUNDER_PHONE);
      assert.fail('should have thrown');
    } catch (err) {
      assert.ok(err instanceof DuplicatePhoneError);
      const msg = err.message;
      // The full 26-char user_id must NOT appear; only the prefix.
      assert.equal(msg.includes('founder-uuid-long-12345678'), false);
      // The phone plaintext must NEVER appear.
      assert.equal(msg.includes('5550100001'), false);
      assert.equal(msg.includes(FOUNDER_PHONE), false);
    }
  });

  it('does NOT throw DuplicatePhoneError when the SAME user re-sets their own phone (idempotency)', async () => {
    const store = makeStore();
    await store.setPhone('founder', FOUNDER_PHONE);
    // Should NOT throw — same user_id + same hash = idempotent.
    await store.setPhone('founder', FOUNDER_PHONE);
    const hash = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    assert.equal(await store.findUserIdByPhoneHash(hash), 'founder');
  });

  it('allows a user to change phones (releases their old hash; old hash resolves to null)', async () => {
    const store = makeStore();
    await store.setPhone('founder', FOUNDER_PHONE);
    await store.setPhone('founder', FRIEND_PHONE_2);
    const oldHash = hashPhone(FOUNDER_PHONE, phoneHashConfig);
    const newHash = hashPhone(FRIEND_PHONE_2, phoneHashConfig);
    assert.equal(await store.findUserIdByPhoneHash(oldHash), null);
    assert.equal(await store.findUserIdByPhoneHash(newHash), 'founder');
    assert.equal(await store.getPhoneForUser('founder'), FRIEND_PHONE_2);
  });

  it('isFounder returns true ONLY for explicitly marked founder rows', async () => {
    const store = makeStore();
    await store.setPhone('founder', FOUNDER_PHONE);
    await store.setPhone('friend-1', FRIEND_PHONE);
    assert.equal(await store.isFounder('founder'), false); // not yet marked
    store.markFounder('founder');
    assert.equal(await store.isFounder('founder'), true);
    assert.equal(await store.isFounder('friend-1'), false);
  });
});

/* ---------------------------------------------------------------------- */
/* loadPhoneHashConfig — env contract                                     */
/* ---------------------------------------------------------------------- */

describe('loadPhoneHashConfig', () => {
  it('loads a 32-byte base64 key from BREVIO_PHONE_HASH_KEY', () => {
    const cfg = withEnv({ BREVIO_PHONE_HASH_KEY: TEST_HASH_KEY, BREVIO_DEV_MODE: undefined }, () =>
      loadPhoneHashConfig(process.env)
    );
    assert.equal(cfg.hmacKey.length, 32);
  });

  it('accepts hex: prefix for the HMAC key', () => {
    const hexKey = 'hex:' + Buffer.alloc(32, 5).toString('hex');
    const cfg = withEnv({ BREVIO_PHONE_HASH_KEY: hexKey, BREVIO_DEV_MODE: undefined }, () =>
      loadPhoneHashConfig(process.env)
    );
    assert.equal(cfg.hmacKey.length, 32);
  });

  it('refuses to boot when BREVIO_PHONE_HASH_KEY is missing in production', () => {
    withEnv({ BREVIO_PHONE_HASH_KEY: undefined, BREVIO_DEV_MODE: undefined }, () => {
      assert.throws(() => loadPhoneHashConfig(process.env), /BREVIO_PHONE_HASH_KEY required/);
    });
  });

  it('falls back to a per-process random key in BREVIO_DEV_MODE=true', () => {
    withEnv({ BREVIO_PHONE_HASH_KEY: undefined, BREVIO_DEV_MODE: 'true' }, () => {
      const cfg = loadPhoneHashConfig(process.env);
      assert.equal(cfg.hmacKey.length, 32);
    });
  });

  it('rejects keys that are not exactly 32 bytes', () => {
    const shortKey = Buffer.alloc(16, 1).toString('base64');
    withEnv({ BREVIO_PHONE_HASH_KEY: shortKey, BREVIO_DEV_MODE: undefined }, () => {
      assert.throws(() => loadPhoneHashConfig(process.env), /must decode to exactly 32 bytes/);
    });
  });
});
