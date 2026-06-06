// Phase v0.5.9 — Feedback + Learn/Grow Loop consumer side (Brevio-wide).
//
// applyFeedback() is the "Learn/Grow" half of the substrate: it reads a
// freshly-written feedback_event and decides whether to upsert a
// memory_signal. v0.5.9 ships exactly ONE hardcoded match arm (founder
// lock Q5.B — see memory project_v05-9-scope):
//
//   IF source_surface == 'email_alert'
//      AND mapped_verb == 'ignored'             (kind itself OR via mapLegacyFeedbackKind)
//      AND detail.dimension == 'sender'         (caller-supplied OR via mapping overlay)
//      AND a sender identifier is present
//   THEN
//      upsert memory_signals(
//        kind='sender_feedback_ignored',
//        scope_key=<HMAC-SHA-256 hash of sender_email>
//      )
//      emit `brevio.feedback.applied` audit row
//
// All other (source_surface, kind, dimension) tuples return { kind:
// 'no_match' } — applyFeedback is a no-op, no audit, no memory write.
//
// Future-extension shape: a switch/match block on (source_surface, verb,
// dimension). New surfaces (calendar_reminder, draft_suggestion, etc.) add
// new arms in their own 6Q-gated phases. No premature rule-engine
// abstraction — per founder lock [[real-or-absent-no-half-wired]] only the
// real, exercised rules ship.
//
// PRIVACY GUARDRAIL (founder-locked at v0.5.9 approval time):
//   * `sender_feedback_ignored` memory_signal scope_key is the HMAC hash
//     (NOT the raw sender_email)
//   * `sender_feedback_ignored` memory_signal detail contains NO raw
//     sender_email
//   * `brevio.feedback.applied` audit detail contains NO raw sender_email
//   * The HMAC input is `user_id + ':' + email.trim().toLowerCase()` so
//     user_id participation prevents cross-user enumeration even with key
//     compromise
//   * No PIL ranking consumption: ranker does NOT read this signal in
//     v0.5.9. Write-only this phase.

import { createHmac } from 'node:crypto';

import type { AuditStore } from '../core/audit.js';
import type {
  BrevioFeedbackEventKind,
  BrevioFeedbackSurface,
  FeedbackEvent
} from './feedback-events.js';
import { resolveFeedbackVerb, mapLegacyFeedbackKind } from './feedback-events.js';
import type { MemorySignalStore } from './memory-signals.js';

/* ---------------------------------------------------------------------- */
/* Sender-email hashing                                                   */
/* ---------------------------------------------------------------------- */

// Normalize an email for hashing. Lowercases + trims. Does NOT strip
// `+aliases` (founder lock: alias stripping would silently merge users'
// expectations about distinct senders). Returns empty string on null/
// undefined so the caller can detect "no sender provided."
export function normalizeEmailForHash(email: string | null | undefined): string {
  if (typeof email !== 'string') return '';
  return email.trim().toLowerCase();
}

// Compute the per-user, privacy-preserving scope_key for a sender. The
// user_id participation in the MAC input means a leaked key still does NOT
// allow cross-user enumeration of "is sender X ignored by user Y?" without
// knowing user Y's id. Truncated to 32 hex chars (128 bits) — collision-
// safe even at extreme scale, smaller index footprint than full 64 hex.
export function hashSenderKey(
  userId: string,
  email: string,
  senderHashKey: Buffer
): string {
  const normalized = normalizeEmailForHash(email);
  return createHmac('sha256', senderHashKey)
    .update(userId + ':' + normalized)
    .digest('hex')
    .slice(0, 32);
}

/* ---------------------------------------------------------------------- */
/* Result type                                                            */
/* ---------------------------------------------------------------------- */

export type AppliedFeedbackResult =
  | {
      readonly kind: 'applied';
      readonly memory_signal_kind: 'sender_feedback_ignored';
      readonly memory_signal_action: 'created' | 'updated';
      readonly scope_key_hash: string;
      readonly ignored_count: number;
      readonly confidence: number;
    }
  | { readonly kind: 'no_match' };

/* ---------------------------------------------------------------------- */
/* Deps                                                                   */
/* ---------------------------------------------------------------------- */

export interface ApplyFeedbackDeps {
  readonly memoryStore: MemorySignalStore;
  readonly auditStore: AuditStore;
  // HMAC-SHA-256 key for scope_key derivation. Loaded from
  // BREVIO_SENDER_HASH_KEY env var at startup (32 bytes, separate hash
  // domain from BREVIO_TOKEN_KEK / BREVIO_PHONE_HASH_KEY).
  readonly senderHashKey: Buffer;
  // Override hooks for testability.
  readonly now?: () => number;
}

/* ---------------------------------------------------------------------- */
/* Internal helpers                                                       */
/* ---------------------------------------------------------------------- */

function getString(detail: Record<string, unknown> | null | undefined, key: string): string | undefined {
  if (!detail) return undefined;
  const v = (detail as Record<string, unknown>)[key];
  return typeof v === 'string' ? v : undefined;
}

function getNumberArray(detail: Record<string, unknown> | null | undefined, key: string): number[] {
  if (!detail) return [];
  const v = (detail as Record<string, unknown>)[key];
  if (!Array.isArray(v)) return [];
  return (v as unknown[]).filter((x): x is number => typeof x === 'number' && Number.isFinite(x));
}

function getNumber(detail: Record<string, unknown> | null | undefined, key: string): number | undefined {
  if (!detail) return undefined;
  const v = (detail as Record<string, unknown>)[key];
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined;
}

// Confidence derivation per Q5.B founder lock: deterministic, no LLM, no
// probabilistic model. min(0.5 + 0.1 * ignored_count, 0.95).
function deriveConfidence(ignored_count: number): number {
  return Math.min(0.5 + 0.1 * ignored_count, 0.95);
}

// Cap the source_feedback_event_ids array so a long-lived signal doesn't
// grow unbounded. 100 is generous; PIL phase may rebalance.
const SOURCE_EVENT_IDS_CAP = 100;

/* ---------------------------------------------------------------------- */
/* applyFeedback                                                          */
/* ---------------------------------------------------------------------- */

export async function applyFeedback(
  event: FeedbackEvent,
  deps: ApplyFeedbackDeps
): Promise<AppliedFeedbackResult> {
  // v0.5.9 LOCKED match arm: (email_alert, ignored, sender).
  if (event.source_surface !== 'email_alert') {
    return { kind: 'no_match' };
  }
  const verb: BrevioFeedbackEventKind | null = resolveFeedbackVerb(event.kind);
  if (verb !== 'ignored') {
    return { kind: 'no_match' };
  }
  // Derive dimension from caller's detail OR the legacy-kind mapping overlay.
  // For the legacy `ignored_sender` kind, the mapping overlay carries
  // dimension='sender'; for the generic `ignored` kind, the caller must
  // supply detail.dimension='sender' explicitly.
  const detailDimension = getString(event.detail, 'dimension');
  const overlayDimension = mapLegacyFeedbackKind(event.kind)?.overlay.dimension;
  const effectiveDimension = detailDimension ?? overlayDimension;
  if (effectiveDimension !== 'sender') {
    return { kind: 'no_match' };
  }
  // Sender identifier — prefer the dedicated column; fall back to detail.target.
  const senderRaw = event.sender_email ?? getString(event.detail, 'target') ?? '';
  if (!senderRaw) {
    return { kind: 'no_match' };
  }

  // PRIVACY GATE: hash the sender BEFORE any subsequent operation. The raw
  // sender never enters memory_signal detail or audit detail past this line.
  const scope_key = hashSenderKey(event.user_id, senderRaw, deps.senderHashKey);

  const nowMs = (deps.now ?? Date.now)();
  const nowIso = new Date(nowMs).toISOString();

  // Read existing signal to compute the update shape.
  const existing = await deps.memoryStore.get(event.user_id, 'sender_feedback_ignored', scope_key);
  const priorDetail = (existing?.detail ?? null) as Record<string, unknown> | null;
  const priorIgnoredCount = getNumber(priorDetail, 'ignored_count') ?? 0;
  const priorFirstAt = getString(priorDetail, 'first_ignored_at');
  const priorEventIds = getNumberArray(priorDetail, 'source_feedback_event_ids');

  const ignored_count = priorIgnoredCount + 1;
  const first_ignored_at = priorFirstAt ?? nowIso;
  const last_ignored_at = nowIso;
  // Append the new feedback_event_id if present; cap the array length.
  const eventId = event.id;
  const nextEventIds = typeof eventId === 'number' && Number.isFinite(eventId)
    ? [...priorEventIds, eventId].slice(-SOURCE_EVENT_IDS_CAP)
    : priorEventIds;

  const confidence = deriveConfidence(ignored_count);
  const memory_signal_action: 'created' | 'updated' = existing ? 'updated' : 'created';

  await deps.memoryStore.upsert({
    user_id: event.user_id,
    kind: 'sender_feedback_ignored',
    scope_key,
    // Detail is STRUCTURAL ONLY per the founder approval-time privacy
    // guardrail: no raw sender, no subject, no body. The scope_key (the
    // HMAC hash) is the canonical sender identifier in this signal.
    detail: {
      ignored_count,
      first_ignored_at,
      last_ignored_at,
      source_feedback_event_ids: nextEventIds,
      source_surface: 'email_alert' satisfies BrevioFeedbackSurface
    },
    source: 'feedback_derived',
    confidence,
    updated_at: nowIso
  });

  // Emit the consumer-side audit. Structural-only detail per Q6.C lock.
  // Best-effort: if the audit write fails, swallow (consistent with the
  // v0.5.8 event_observed best-effort lock) — the memory_signal upsert is
  // the load-bearing side effect and has already succeeded.
  try {
    await deps.auditStore.write({
      actor_user_id: event.user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'brevio.feedback.applied',
      target: 'memory_signal:sender_feedback_ignored',
      result: 'success',
      detail: {
        feedback_event_id: eventId ?? null,
        source_surface: 'email_alert',
        verb,
        dimension: effectiveDimension,
        memory_signal_kind: 'sender_feedback_ignored',
        memory_signal_action,
        memory_signal_scope_key_hash: scope_key,
        confidence
      },
      occurred_at: nowIso
    });
  } catch {
    // best-effort per Q6.C lock; memory_signal upsert already landed.
  }

  return {
    kind: 'applied',
    memory_signal_kind: 'sender_feedback_ignored',
    memory_signal_action,
    scope_key_hash: scope_key,
    ignored_count,
    confidence
  };
}

/* ---------------------------------------------------------------------- */
/* Loader for BREVIO_SENDER_HASH_KEY                                       */
/* ---------------------------------------------------------------------- */

// Parse a 32-byte key from the env var (base64 or hex: prefix). Mirrors
// the existing loadCryptoConfig pattern in security/token-crypto.ts.
// Throws on missing or under-length input — preflight catches this earlier
// at boot time so the runtime should never see a missing key.
export function loadSenderHashKey(env: NodeJS.ProcessEnv = process.env): Buffer {
  const raw = (env.BREVIO_SENDER_HASH_KEY ?? '').trim();
  if (!raw) {
    throw new Error(
      'BREVIO_SENDER_HASH_KEY is required for v0.5.9 feedback substrate. ' +
        'Generate with: `openssl rand -base64 32`. NEVER reuse from BREVIO_TOKEN_KEK / BREVIO_PHONE_HASH_KEY (separate hash domain).'
    );
  }
  let decoded: Buffer;
  try {
    decoded = raw.startsWith('hex:') ? Buffer.from(raw.slice(4), 'hex') : Buffer.from(raw, 'base64');
  } catch (err) {
    throw new Error(
      `BREVIO_SENDER_HASH_KEY is not a valid base64 or hex: value (${err instanceof Error ? err.message : String(err)})`
    );
  }
  if (decoded.length < 32) {
    throw new Error(
      `BREVIO_SENDER_HASH_KEY decoded length ${decoded.length} bytes, need ≥ 32`
    );
  }
  return decoded;
}
