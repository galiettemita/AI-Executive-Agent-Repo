// Phase v0.5.11 — PIL shadow ranker context (PROJECTION + DECAY ONLY).
//
// HARD INVARIANT — LIVE RANKER MUST NEVER CALL THIS MODULE.
//
// Per founder lock (memory: project_v05-11-scope, hard boundaries #1+#2+#3):
//   - Production ranker call site passes `pil_context: null` UNCONDITIONALLY
//     in v0.5.11. No env switch flips it on.
//   - This module is consumed ONLY by the offline eval harness
//     (apps/fomo/src/eval/pil-shadow.eval.ts) and unit tests.
//   - When v0.5.12 wires this into the live ranker, it does so under its own
//     6Q gate.
//
// Three responsibilities:
//   1. Look up `sender_importance` + `sender_suppressed` for (user_id,
//      scope_key=HMAC(sender_email)).
//   2. Apply Q3.B linear recency decay (full 0-90d, linear 90-180d, zero
//      180d+) to the importance score at READ time. Aggregator writes raw
//      score; reader decays.
//   3. Project into the structured `PilContext` shape the future ranker
//      prompt builder will accept.
//
// Privacy: returns numeric + boolean fields only. NEVER raw sender_email,
// subject, body, headers. The caller already holds the HMAC scope_key.

import { type MemorySignalStore } from '../memory/memory-signals.js';

export interface PilContext {
  /**
   * Linear-decayed sender importance score in [-1.0, +1.0]. Decay factor:
   *   age <= recency_full_days       → factor = 1.0
   *   age <= full + decay window     → factor linear 1.0 → 0.0
   *   age >  full + decay window     → factor = 0.0
   * Aggregate score is treated as an event at `last_updated`; this is
   * approximate but well-defined and testable. Per-event decay is documented
   * as out of v0.5.11 scope.
   */
  readonly sender_importance_score: number;
  /** Sum of n_positive_events + n_negative_events on the underlying signal. */
  readonly sender_importance_n_events: number;
  /** True when sender_suppressed memory_signal exists with detail.suppressed=true. */
  readonly sender_suppressed: boolean;
  /** ISO 8601 of the most recent underlying memory_signal update, or null. */
  readonly last_updated: string | null;
  /** For audit traceability: which decay factor (0.0–1.0) was applied. */
  readonly decay_factor_applied: number;
}

export interface PilContextDeps {
  readonly memoryStore: MemorySignalStore;
  /** Tunable: full-weight window in days. Default 90 per Q3.B. */
  readonly recency_full_days: number;
  /** Tunable: linear-decay tail in days (0 = hard cliff). Default 90 per Q3.B. */
  readonly recency_decay_days: number;
  /** Override for tests; default = Date.now(). */
  readonly now?: () => Date;
}

/**
 * Build the PIL shadow context for a specific (user_id, sender_email_hash).
 *
 * Returns `null` when:
 *   - `senderEmailHash` is null (the alert had no privacy-safe sender hash
 *     because it was created pre-v0.5.11 migration OR by a fixture that
 *     didn't supply sender_email);
 *   - both `sender_importance` and `sender_suppressed` are absent for that
 *     (user_id, scope_key). Cross-user keyspace isolation is guaranteed
 *     because hashSenderKey includes user_id in the HMAC input — User A's
 *     scope_key cannot collide with User B's lookup for the same raw sender.
 */
export async function buildPilContext(
  userId: string,
  senderEmailHash: string | null,
  deps: PilContextDeps
): Promise<PilContext | null> {
  if (senderEmailHash === null || senderEmailHash === '') {
    return null;
  }
  const [importance, suppressed] = await Promise.all([
    deps.memoryStore.get(userId, 'sender_importance', senderEmailHash),
    deps.memoryStore.get(userId, 'sender_suppressed', senderEmailHash)
  ]);
  if (importance === null && suppressed === null) {
    return null;
  }

  const importanceDetail = (importance?.detail ?? {}) as {
    score?: number;
    n_positive_events?: number;
    n_negative_events?: number;
    last_updated?: string;
  };
  const suppressedDetail = (suppressed?.detail ?? {}) as {
    suppressed?: boolean;
    set_at?: string;
  };

  const rawScore = typeof importanceDetail.score === 'number' ? importanceDetail.score : 0;
  const nPos = importanceDetail.n_positive_events ?? 0;
  const nNeg = importanceDetail.n_negative_events ?? 0;

  // Decay basis: prefer importance.last_updated; fall back to memory_signal
  // updated_at; fall back to suppressed.set_at when there's no importance row.
  const decayBasisIso =
    importanceDetail.last_updated ??
    importance?.updated_at ??
    suppressedDetail.set_at ??
    suppressed?.updated_at ??
    null;

  const now = deps.now ? deps.now() : new Date();
  const decayFactor = computeDecayFactor(
    decayBasisIso,
    now,
    deps.recency_full_days,
    deps.recency_decay_days
  );
  const decayedScore = rawScore * decayFactor;

  return Object.freeze({
    sender_importance_score: decayedScore,
    sender_importance_n_events: nPos + nNeg,
    sender_suppressed: suppressedDetail.suppressed === true,
    last_updated:
      importance?.updated_at ?? suppressed?.updated_at ?? null,
    decay_factor_applied: decayFactor
  });
}

/* ====================================================================== */
/* Phase v0.5.12 — buildLivePilContext (live ranker read path)            */
/* ====================================================================== */

/**
 * Canonical HMAC scope_key shape produced by hashSenderKey(): 32 lowercase
 * hex characters. v0.5.10 `applyIgnoreSender` writes a placeholder
 * `scope_key='message:<id>'` row that v0.5.12 MUST ignore — see founder
 * rule in [[v05-12-scope]] "Critical implementation rule (READ-SIDE FILTER)".
 */
export const CANONICAL_SCOPE_KEY_REGEX = /^[a-f0-9]{32}$/;

/**
 * Phase v0.5.12 — live ranker read path.
 *
 * Differences from buildPilContext (the shadow projection consumer):
 *   1. Adds a READ-SIDE FILTER: scope_keys that do not match the canonical
 *      32-hex HMAC shape return null. v0.5.10 `applyIgnoreSender`'s legacy
 *      `scope_key='message:<id>'` placeholder rows are ignored. BB6 fixture
 *      in pil-live.eval.ts is the LOAD-BEARING coverage.
 *   2. Cross-user safety: the underlying lookup is `(userId, scope_key)`,
 *      and `hashSenderKey()` includes user_id in the HMAC input — User A's
 *      scope_key cannot collide with User B's lookup for the same raw
 *      sender. Founder guardrail 3. BB4 fixture is the LOAD-BEARING
 *      adversarial coverage.
 *   3. Otherwise reuses buildPilContext + computeDecayFactor unchanged —
 *      same projection contract, same Q3.B decay (no v0.5.12 divergence).
 *
 * The kill switch (FOMO_PIL_LIVE_ENABLED) is checked AT THE CALL SITE
 * (worker), not inside this module — keeps the function pure for testing.
 * BB7 fixture documents the kill-switch contract at the call site.
 */
export async function buildLivePilContext(
  userId: string,
  senderEmailHash: string | null,
  deps: PilContextDeps
): Promise<PilContext | null> {
  if (senderEmailHash === null || senderEmailHash === '') {
    return null;
  }
  if (!CANONICAL_SCOPE_KEY_REGEX.test(senderEmailHash)) {
    // Legacy placeholder shape (e.g. 'message:<id>') or any other
    // non-canonical scope_key. Founder rule: ignore unconditionally.
    return null;
  }
  return buildPilContext(userId, senderEmailHash, deps);
}

/**
 * Q3.B linear decay. Pure function, exported for unit tests.
 *
 *   age_days <= full_days                            → 1.0
 *   age_days in (full_days, full_days+decay_days]    → linear 1.0 → 0.0
 *   age_days >  full_days + decay_days               → 0.0
 *
 * Tolerates null basis (returns 1.0 — caller treats as fresh) and clamps
 * negative ages (clock skew) to 1.0.
 */
export function computeDecayFactor(
  basisIso: string | null,
  now: Date,
  recencyFullDays: number,
  recencyDecayDays: number
): number {
  if (basisIso === null) return 1.0;
  const basis = Date.parse(basisIso);
  if (!Number.isFinite(basis)) return 1.0;
  const ageMs = now.getTime() - basis;
  if (ageMs <= 0) return 1.0;
  const ageDays = ageMs / (1000 * 60 * 60 * 24);
  if (ageDays <= recencyFullDays) return 1.0;
  if (recencyDecayDays <= 0) return 0.0;
  const decayed = 1.0 - (ageDays - recencyFullDays) / recencyDecayDays;
  if (decayed <= 0) return 0.0;
  if (decayed >= 1) return 1.0;
  return decayed;
}
