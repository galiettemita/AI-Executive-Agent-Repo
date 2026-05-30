// Phase v0.5.1 step #3 — regression tests for the invite-token store.
//
// Founder corrections covered (all from [[multitenant-design-principles]] §5):
//   5a. Store ONLY the token_hash; plaintext NEVER persisted.
//   5b. NEVER log plaintext token (test via Error.message + record shape).
//   5c. Consume AFTER successful flow, not on first GET. The store's
//       `consume()` performs the atomic conditional UPDATE — second
//       call returns InvalidInviteTokenError('consumed'); record's
//       consumed_user_id is the FIRST consumer (replay didn't
//       overwrite).
//   5d. Don't log raw phones (intended_phone_hash is already a hash).
//   5e. Bound to intended_phone_hash (returned from consume, verified
//       by /onboard against the friend's resolved phone hash).
//
// Distinct synthetic phone hashes (correction #4) — never reuse the
// founder's real number.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  InMemoryInviteTokenStore,
  InvalidInviteTokenError,
  generateTokenPlaintext,
  hashToken,
  tokenHashPrefix,
  tokenHashesEqual
} from './invite-tokens.js';
import { type CryptoConfig } from './token-crypto.js';
import {
  decryptInviteBoundPhone,
  encryptInviteBoundPhone,
  hashPhone,
  type PhoneEnvelopeJson,
  type PhoneHashConfig
} from './phone-allowlist.js';

const FOUNDER_PHONE_HASH = 'a'.repeat(64);
const FRIEND_PHONE_HASH = 'b'.repeat(64);

// Step 4.1 — encrypted-envelope fixtures bound to the test phone
// hashes via AAD. Real production envelopes come from
// encryptInviteBoundPhone(plaintext, intended_phone_hash, crypto).
const TEST_KEK_INVITE = Buffer.alloc(32, 7).toString('base64');
const TEST_HASH_KEY_INVITE = Buffer.alloc(32, 31).toString('base64');
const cryptoConfigInvite: CryptoConfig = {
  kek: Buffer.from(TEST_KEK_INVITE, 'base64'),
  devMode: false
};
const phoneHashConfigInvite: PhoneHashConfig = {
  hmacKey: Buffer.from(TEST_HASH_KEY_INVITE, 'base64')
};

// A real friend E.164 + real envelope bound to the real hash, so
// the round-trip property tests (Step 4.1) can verify the hash
// matches what the route would re-compute at callback time.
const STEP_41_FRIEND_PHONE = '+15550100002';
const STEP_41_FRIEND_HASH = hashPhone(STEP_41_FRIEND_PHONE, phoneHashConfigInvite);
const STEP_41_FRIEND_ENVELOPE: PhoneEnvelopeJson = encryptInviteBoundPhone(
  STEP_41_FRIEND_PHONE,
  STEP_41_FRIEND_HASH,
  cryptoConfigInvite
);

// For the legacy step-3 tests that use FRIEND_PHONE_HASH = 'b'.repeat(64) —
// build a real ciphertext bound to that synthetic hash so the
// envelope's AAD round-trip succeeds inside the assertions.
const STEP_41_LEGACY_ENVELOPE: PhoneEnvelopeJson = encryptInviteBoundPhone(
  '+15550199998',
  FRIEND_PHONE_HASH,
  cryptoConfigInvite
);

/* ---------------------------------------------------------------------- */
/* Token generation + hashing primitives                                  */
/* ---------------------------------------------------------------------- */

describe('generateTokenPlaintext', () => {
  it('produces a 43-character URL-safe base64 string (32 bytes → 256 bits)', () => {
    const t = generateTokenPlaintext();
    assert.equal(t.length, 43);
    assert.match(t, /^[A-Za-z0-9_-]+$/);
    // No padding characters.
    assert.equal(t.includes('='), false);
    assert.equal(t.includes('+'), false);
    assert.equal(t.includes('/'), false);
  });

  it('produces a different token on every call (entropy invariant)', () => {
    const seen = new Set<string>();
    for (let i = 0; i < 50; i++) seen.add(generateTokenPlaintext());
    assert.equal(seen.size, 50);
  });
});

describe('hashToken', () => {
  it('produces a 64-char hex sha256 digest', () => {
    const h = hashToken(generateTokenPlaintext());
    assert.equal(h.length, 64);
    assert.match(h, /^[0-9a-f]{64}$/);
  });

  it('is deterministic — same plaintext always yields same hash', () => {
    const t = generateTokenPlaintext();
    assert.equal(hashToken(t), hashToken(t));
  });

  it('different plaintexts yield different hashes', () => {
    assert.notEqual(hashToken(generateTokenPlaintext()), hashToken(generateTokenPlaintext()));
  });

  it('hash output does NOT contain the plaintext (string-search check)', () => {
    const t = generateTokenPlaintext();
    const h = hashToken(t);
    assert.equal(h.includes(t), false);
    assert.equal(h.includes(t.slice(0, 10)), false);
  });
});

describe('tokenHashesEqual', () => {
  it('returns true for equal hashes', () => {
    const h = hashToken('plaintext');
    assert.equal(tokenHashesEqual(h, h), true);
  });
  it('returns false for different hashes', () => {
    assert.equal(tokenHashesEqual(hashToken('a'), hashToken('b')), false);
  });
  it('returns false for hashes of different lengths', () => {
    assert.equal(tokenHashesEqual('a'.repeat(64), 'a'.repeat(63)), false);
  });
});

describe('tokenHashPrefix', () => {
  it('returns the first 8 chars of the hash (audit-safe identifier)', () => {
    const h = hashToken('plaintext');
    assert.equal(tokenHashPrefix(h), h.slice(0, 8));
    assert.equal(tokenHashPrefix(h).length, 8);
  });
});

/* ---------------------------------------------------------------------- */
/* InMemoryInviteTokenStore.issue                                         */
/* ---------------------------------------------------------------------- */

describe('InviteTokenStore.issue', () => {
  it('returns plaintext + id + hash + expires_at; stores ONLY the hash internally', async () => {
    const store = new InMemoryInviteTokenStore();
    const issued = await store.issue({
      intended_phone_hash: FRIEND_PHONE_HASH,
      intended_phone_encrypted: STEP_41_LEGACY_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    assert.equal(typeof issued.token_plaintext, 'string');
    assert.equal(typeof issued.id, 'number');
    assert.equal(typeof issued.token_hash, 'string');
    assert.ok(issued.expires_at instanceof Date);
    // The hash on the issued result matches hashToken(plaintext).
    assert.equal(issued.token_hash, hashToken(issued.token_plaintext));

    // The store record contains ONLY the hash. We assert by looking
    // up via the hash (works) AND searching the stringified record
    // for the plaintext (must NOT appear).
    const record = await store.lookupByHash(issued.token_hash);
    assert.ok(record);
    const dump = JSON.stringify(record);
    assert.equal(dump.includes(issued.token_plaintext), false, 'plaintext token leaked into record');
    assert.equal(record.token_hash, issued.token_hash);
    assert.equal(record.intended_phone_hash, FRIEND_PHONE_HASH);
    assert.equal(record.issued_by_user_id, 'founder');
    assert.equal(record.consumed_at, null);
    assert.equal(record.consumed_user_id, null);
  });

  it('expires_at is 24 hours after now by default', async () => {
    const store = new InMemoryInviteTokenStore();
    const now = new Date('2026-06-01T00:00:00Z');
    const issued = await store.issue(
      {
        intended_phone_hash: FRIEND_PHONE_HASH,
        intended_phone_encrypted: STEP_41_LEGACY_ENVELOPE,
        issued_by_user_id: 'founder'
      },
      now
    );
    const expected = new Date(now.getTime() + 24 * 60 * 60 * 1000);
    assert.equal(issued.expires_at.getTime(), expected.getTime());
  });

  it('accepts a custom ttl_ms', async () => {
    const store = new InMemoryInviteTokenStore();
    const now = new Date('2026-06-01T00:00:00Z');
    const issued = await store.issue(
      {
        intended_phone_hash: FRIEND_PHONE_HASH,
        intended_phone_encrypted: STEP_41_LEGACY_ENVELOPE,
        issued_by_user_id: 'founder',
        ttl_ms: 60_000
      },
      now
    );
    assert.equal(issued.expires_at.getTime(), now.getTime() + 60_000);
  });

  it('multiple issues for the same intended phone are allowed (each is its own row)', async () => {
    const store = new InMemoryInviteTokenStore();
    const a = await store.issue({
      intended_phone_hash: FRIEND_PHONE_HASH,
      intended_phone_encrypted: STEP_41_LEGACY_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    const b = await store.issue({
      intended_phone_hash: FRIEND_PHONE_HASH,
      intended_phone_encrypted: STEP_41_LEGACY_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    assert.notEqual(a.id, b.id);
    assert.notEqual(a.token_hash, b.token_hash);
  });
});

/* ---------------------------------------------------------------------- */
/* InviteTokenStore.lookupByHash                                          */
/* ---------------------------------------------------------------------- */

describe('InviteTokenStore.lookupByHash', () => {
  it('returns the record for a known hash', async () => {
    const store = new InMemoryInviteTokenStore();
    const issued = await store.issue({
      intended_phone_hash: FRIEND_PHONE_HASH,
      intended_phone_encrypted: STEP_41_LEGACY_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    const record = await store.lookupByHash(issued.token_hash);
    assert.ok(record);
    assert.equal(record.id, issued.id);
  });

  it('returns null for an unknown hash', async () => {
    const store = new InMemoryInviteTokenStore();
    const record = await store.lookupByHash('z'.repeat(64));
    assert.equal(record, null);
  });

  it('does NOT consume the token (correction #5c — lookup is read-only)', async () => {
    const store = new InMemoryInviteTokenStore();
    const issued = await store.issue({
      intended_phone_hash: FRIEND_PHONE_HASH,
      intended_phone_encrypted: STEP_41_LEGACY_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    // Three lookups should all succeed.
    for (let i = 0; i < 3; i++) {
      const r = await store.lookupByHash(issued.token_hash);
      assert.ok(r);
      assert.equal(r.consumed_at, null);
    }
    // Subsequent consume still works — lookups did not consume.
    const consumed = await store.consume({
      token_hash: issued.token_hash,
      consumed_user_id: 'friend-1'
    });
    assert.equal(consumed.intended_phone_hash, FRIEND_PHONE_HASH);
  });
});

/* ---------------------------------------------------------------------- */
/* InviteTokenStore.consume — atomic, one-time, expiring, bound           */
/* ---------------------------------------------------------------------- */

describe('InviteTokenStore.consume (Phase v0.5.1 invite-flow correctness)', () => {
  it('consumes a valid token on first call; returns id + intended_phone_hash', async () => {
    const store = new InMemoryInviteTokenStore();
    const issued = await store.issue({
      intended_phone_hash: FRIEND_PHONE_HASH,
      intended_phone_encrypted: STEP_41_LEGACY_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    const result = await store.consume({
      token_hash: issued.token_hash,
      consumed_user_id: 'friend-1'
    });
    assert.equal(result.id, issued.id);
    assert.equal(result.intended_phone_hash, FRIEND_PHONE_HASH);

    // Record now reflects consumption.
    const record = await store.lookupByHash(issued.token_hash);
    assert.ok(record);
    assert.ok(record.consumed_at instanceof Date);
    assert.equal(record.consumed_user_id, 'friend-1');
  });

  it('second consume throws InvalidInviteTokenError("consumed") — token is one-time (correction #5c)', async () => {
    const store = new InMemoryInviteTokenStore();
    const issued = await store.issue({
      intended_phone_hash: FRIEND_PHONE_HASH,
      intended_phone_encrypted: STEP_41_LEGACY_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    await store.consume({ token_hash: issued.token_hash, consumed_user_id: 'friend-1' });

    let thrown: unknown;
    try {
      await store.consume({ token_hash: issued.token_hash, consumed_user_id: 'friend-replay' });
    } catch (err) {
      thrown = err;
    }
    assert.ok(thrown instanceof InvalidInviteTokenError);
    assert.equal((thrown as InvalidInviteTokenError).reason, 'consumed');

    // Replay did NOT overwrite the first consumer.
    const record = await store.lookupByHash(issued.token_hash);
    assert.equal(record?.consumed_user_id, 'friend-1');
  });

  it('consume throws InvalidInviteTokenError("expired") for a past-expiry token', async () => {
    const store = new InMemoryInviteTokenStore();
    const issuedAt = new Date('2026-06-01T00:00:00Z');
    const issued = await store.issue(
      {
        intended_phone_hash: FRIEND_PHONE_HASH,
        intended_phone_encrypted: STEP_41_LEGACY_ENVELOPE,
        issued_by_user_id: 'founder',
        ttl_ms: 1000
      },
      issuedAt
    );
    // Now is 10s past expiry.
    const tenSecondsAfterExpiry = new Date(issuedAt.getTime() + 1000 + 10_000);

    let thrown: unknown;
    try {
      await store.consume({
        token_hash: issued.token_hash,
        consumed_user_id: 'friend-late',
        now: tenSecondsAfterExpiry
      });
    } catch (err) {
      thrown = err;
    }
    assert.ok(thrown instanceof InvalidInviteTokenError);
    assert.equal((thrown as InvalidInviteTokenError).reason, 'expired');

    // Token row remains UNCONSUMED (expired-and-unused state, not silently absorbed).
    const record = await store.lookupByHash(issued.token_hash);
    assert.equal(record?.consumed_at, null);
    assert.equal(record?.consumed_user_id, null);
  });

  it('consume throws InvalidInviteTokenError("unknown") for a hash that was never issued', async () => {
    const store = new InMemoryInviteTokenStore();
    let thrown: unknown;
    try {
      await store.consume({ token_hash: 'z'.repeat(64), consumed_user_id: 'whoever' });
    } catch (err) {
      thrown = err;
    }
    assert.ok(thrown instanceof InvalidInviteTokenError);
    assert.equal((thrown as InvalidInviteTokenError).reason, 'unknown');
  });

  it('returned intended_phone_hash is the binding the /onboard callback must verify (correction #5e)', async () => {
    const store = new InMemoryInviteTokenStore();
    const issued = await store.issue({
      intended_phone_hash: FRIEND_PHONE_HASH,
      intended_phone_encrypted: STEP_41_LEGACY_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    const result = await store.consume({
      token_hash: issued.token_hash,
      consumed_user_id: 'friend-1'
    });
    // The /onboard callback verifies this matches the friend's
    // resolved phone hash. If it doesn't match, the callback rolls
    // back user creation and emits a fomo.onboard.phone_mismatch
    // audit event (wired in step 4).
    assert.equal(result.intended_phone_hash, FRIEND_PHONE_HASH);
    assert.notEqual(result.intended_phone_hash, FOUNDER_PHONE_HASH);
  });
});

/* ---------------------------------------------------------------------- */
/* Privacy invariants                                                     */
/* ---------------------------------------------------------------------- */

describe('Invite-token privacy invariants (correction #5b)', () => {
  it('InvalidInviteTokenError message does NOT include the token_hash or plaintext', () => {
    const err = new InvalidInviteTokenError('consumed');
    assert.equal(err.message.length < 100, true, 'error message must be short / categorical');
    assert.match(err.message, /InvalidInviteTokenError: consumed/);
  });

  it('record.toString / JSON.stringify never includes the plaintext token', async () => {
    const store = new InMemoryInviteTokenStore();
    const issued = await store.issue({
      intended_phone_hash: FRIEND_PHONE_HASH,
      intended_phone_encrypted: STEP_41_LEGACY_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    const record = await store.lookupByHash(issued.token_hash);
    assert.ok(record);
    assert.equal(JSON.stringify(record).includes(issued.token_plaintext), false);
  });
});

/* ---------------------------------------------------------------------- */
/* Step 4.1 — invite phone encrypted binding (no half-wired)              */
/* ---------------------------------------------------------------------- */

describe('Step 4.1 — invite phone encrypted binding', () => {
  it('issue() rejects an invite without a non-empty encrypted envelope (REAL or ABSENT)', async () => {
    const store = new InMemoryInviteTokenStore();
    // @ts-expect-error — runtime guard for missing envelope
    await assert.rejects(
      () =>
        store.issue({
          intended_phone_hash: STEP_41_FRIEND_HASH,
          issued_by_user_id: 'founder'
        }),
      /intended_phone_encrypted envelope is required/
    );
    // @ts-expect-error — runtime guard for empty ciphertext
    await assert.rejects(
      () =>
        store.issue({
          intended_phone_hash: STEP_41_FRIEND_HASH,
          intended_phone_encrypted: { v: 1, ct: '' },
          issued_by_user_id: 'founder'
        }),
      /intended_phone_encrypted envelope is required/
    );
  });

  it('issue() stores the encrypted envelope, NOT the plaintext phone (correction #1)', async () => {
    const store = new InMemoryInviteTokenStore();
    const issued = await store.issue({
      intended_phone_hash: STEP_41_FRIEND_HASH,
      intended_phone_encrypted: STEP_41_FRIEND_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    const record = await store.lookupByHash(issued.token_hash);
    assert.ok(record);
    assert.ok(record.intended_phone_encrypted);
    assert.equal(record.intended_phone_encrypted?.v, 1);
    assert.equal(record.intended_phone_encrypted?.ct, STEP_41_FRIEND_ENVELOPE.ct);
    // The PLAINTEXT phone must NEVER appear anywhere in the record dump.
    const dump = JSON.stringify(record);
    assert.equal(dump.includes(STEP_41_FRIEND_PHONE), false, 'plaintext phone leaked into record');
    assert.equal(dump.includes('5550100002'), false, 'phone digits leaked into record');
  });

  it('encrypted envelope round-trips: callback can decrypt at consume time', async () => {
    const store = new InMemoryInviteTokenStore();
    const issued = await store.issue({
      intended_phone_hash: STEP_41_FRIEND_HASH,
      intended_phone_encrypted: STEP_41_FRIEND_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    const consumed = await store.consume({
      token_hash: issued.token_hash,
      consumed_user_id: 'friend-1'
    });
    assert.ok(consumed.intended_phone_encrypted);
    const decrypted = decryptInviteBoundPhone(
      consumed.intended_phone_encrypted,
      consumed.intended_phone_hash,
      cryptoConfigInvite
    );
    assert.equal(decrypted, STEP_41_FRIEND_PHONE);
    // Hash round-trip — decrypted plaintext re-hashes to the stored hash.
    assert.equal(hashPhone(decrypted, phoneHashConfigInvite), consumed.intended_phone_hash);
  });

  it('TAMPERED ciphertext fails AEAD verification (fail-closed)', async () => {
    const store = new InMemoryInviteTokenStore();
    const issued = await store.issue({
      intended_phone_hash: STEP_41_FRIEND_HASH,
      intended_phone_encrypted: STEP_41_FRIEND_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    const consumed = await store.consume({
      token_hash: issued.token_hash,
      consumed_user_id: 'friend-1'
    });
    assert.ok(consumed.intended_phone_encrypted);
    // Flip a byte in the ciphertext base64. AEAD must reject.
    const tampered = consumed.intended_phone_encrypted.ct;
    const flipped =
      tampered.charAt(0) === 'A' ? 'B' + tampered.slice(1) : 'A' + tampered.slice(1);
    assert.throws(() =>
      decryptInviteBoundPhone(
        { v: consumed.intended_phone_encrypted!.v, ct: flipped },
        consumed.intended_phone_hash,
        cryptoConfigInvite
      )
    );
  });

  it('WRONG AAD (different intended_phone_hash) fails AEAD verification (fail-closed)', async () => {
    const store = new InMemoryInviteTokenStore();
    const issued = await store.issue({
      intended_phone_hash: STEP_41_FRIEND_HASH,
      intended_phone_encrypted: STEP_41_FRIEND_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    const consumed = await store.consume({
      token_hash: issued.token_hash,
      consumed_user_id: 'friend-1'
    });
    // Attempt decrypt with a different hash as AAD — the route's
    // tamper-detection branch fires when intended_phone_hash column
    // is changed independently of the ciphertext column.
    assert.throws(() =>
      decryptInviteBoundPhone(
        consumed.intended_phone_encrypted!,
        'z'.repeat(64), // wrong AAD
        cryptoConfigInvite
      )
    );
  });

  it('decrypted plaintext re-hashes to the stored intended_phone_hash (route invariant)', async () => {
    const store = new InMemoryInviteTokenStore();
    const issued = await store.issue({
      intended_phone_hash: STEP_41_FRIEND_HASH,
      intended_phone_encrypted: STEP_41_FRIEND_ENVELOPE,
      issued_by_user_id: 'founder'
    });
    const consumed = await store.consume({
      token_hash: issued.token_hash,
      consumed_user_id: 'friend-1'
    });
    const decrypted = decryptInviteBoundPhone(
      consumed.intended_phone_encrypted!,
      consumed.intended_phone_hash,
      cryptoConfigInvite
    );
    // This is the exact check the /onboard callback performs before
    // calling phoneAllowlist.setPhone(). It defends against a
    // hypothetical attacker who tampered with the intended_phone_hash
    // column AND re-AAD'd the ciphertext to match.
    const recomputed = hashPhone(decrypted, phoneHashConfigInvite);
    assert.equal(recomputed, consumed.intended_phone_hash);
  });
});
