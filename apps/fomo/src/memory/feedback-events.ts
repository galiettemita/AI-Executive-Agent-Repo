// Feedback Events — discrete events emitted by founder review, by user
// behavior, or by future Brevio surfaces (calendar, drafts, tasks, etc.).
//
// Phase v0.5.9 (Brevio-wide Feedback + Learn/Grow Loop substrate) generalizes
// the original FOMO/email-only design from Phase 2C:
//   * source_surface column on feedback_events discriminates which Brevio
//     surface produced the event (locked enum: BREVIO_FEEDBACK_SURFACES)
//   * BREVIO_FEEDBACK_ACTIVE_SURFACES allowlist gates writes — declared but
//     inactive surfaces (calendar_reminder, draft_suggestion, etc.) are
//     rejected with BrevioFeedbackError so v0.5.9 cannot be "trapped in
//     email" by future architecture drift
//   * BREVIO_FEEDBACK_EVENT_KINDS provides a small generic verb set
//     (approved/rejected/snoozed/ignored/asked_why/corrected) for cross-
//     surface use; the 11 legacy email-shaped kinds remain accepted on
//     write via mapLegacyFeedbackKind for backward compat (kernel test +
//     Slack interactivity continue to pass legacy kinds; storage keeps
//     them literal so countByKind() and existing read paths don't break)
//   * The mapping helper exposes (verb, overlay) for AUDIT/CONSUMER
//     enrichment — `feedback.written` audit detail gains source_surface,
//     verb, dimension, role, legacy_kind; the applyFeedback consumer reads
//     the same (verb, dimension) tuple to decide whether to upsert a
//     memory_signal
//
// Memory-vs-event split (FOMO_DESIGN §14, preserved):
//   * Feedback events are INSTANTANEOUS records ("at time T, user u
//     ignored sender s"). Append-only, never updated.
//   * Memory signals (memory-signals.ts) are CURRENT STATE ("sender s is
//     in the ignored-feedback set for user u right now"). They get
//     upserted as the system learns.
//
// The detail field is redacted with the same safe-logger redact() the
// audit log uses, so a careless caller writing { access_token: 'plain' }
// into a feedback detail does not leak.

import { redact } from '../core/safe-logger.js';

/* ---------------------------------------------------------------------- */
/* Brevio-wide source_surface enum (Q2.A — founder-locked v0.5.9)         */
/* ---------------------------------------------------------------------- */

// All 13 surfaces Brevio promises to support eventually. Declaration order
// is significant: smoke-evidence C3 asserts both presence + count + exact
// declaration of ACTIVE_SURFACES. Adding a new surface here without adding
// it to BREVIO_FEEDBACK_ACTIVE_SURFACES is the "declared but inactive"
// state — writes get rejected at the write gate.
export const BREVIO_FEEDBACK_SURFACES = [
  'email_alert',           // v0.5.9 — FIRST ACTIVE
  'calendar_reminder',     // future
  'draft_suggestion',      // future
  'task_update',           // future
  'stock_watch',           // future
  'coffee_routine',        // future
  'travel_signal',         // future
  'tool_result',           // future
  'browser_summary',       // future
  'booking_preparation',   // future
  'payment_preparation',   // future
  'memory_explanation',    // future
  'why_answer'             // future
] as const;
export type BrevioFeedbackSurface = (typeof BREVIO_FEEDBACK_SURFACES)[number];

export function isBrevioFeedbackSurface(value: unknown): value is BrevioFeedbackSurface {
  return typeof value === 'string' && (BREVIO_FEEDBACK_SURFACES as readonly string[]).includes(value);
}

// The active allowlist. v0.5.9 ships with exactly one entry. Each new
// surface activation runs its own 6Q gate. Smoke-evidence C3 asserts
// exact equality with ['email_alert'].
export const BREVIO_FEEDBACK_ACTIVE_SURFACES = ['email_alert'] as const satisfies readonly BrevioFeedbackSurface[];

export function isActiveFeedbackSurface(value: unknown): value is (typeof BREVIO_FEEDBACK_ACTIVE_SURFACES)[number] {
  return typeof value === 'string' && (BREVIO_FEEDBACK_ACTIVE_SURFACES as readonly string[]).includes(value);
}

/* ---------------------------------------------------------------------- */
/* Brevio-wide generic event-kind taxonomy (Q3.A-modified)                */
/* ---------------------------------------------------------------------- */

// Small generic verb set. Surface-specific meaning lives in `detail`.
// `opened` is intentionally NOT in this array in v0.5.9 — no current
// caller writes `user_opened` and per founder lock we ship only what
// has a real caller. Add 'opened' here when a real caller exists.
export const BREVIO_FEEDBACK_EVENT_KINDS = [
  'approved',
  'rejected',
  'snoozed',
  'ignored',
  'asked_why',
  'corrected'
] as const;
export type BrevioFeedbackEventKind = (typeof BREVIO_FEEDBACK_EVENT_KINDS)[number];

export function isBrevioFeedbackEventKind(value: unknown): value is BrevioFeedbackEventKind {
  return typeof value === 'string' && (BREVIO_FEEDBACK_EVENT_KINDS as readonly string[]).includes(value);
}

/* ---------------------------------------------------------------------- */
/* Legacy 11-kind email-shaped taxonomy (preserved for backward compat)   */
/* ---------------------------------------------------------------------- */

export const FEEDBACK_EVENT_KINDS = [
  'founder_approved',
  'founder_rejected',
  'user_opened',
  'user_snoozed',
  'user_ignored',
  'ignored_sender',
  'asked_why',
  'stop',
  'no_response',
  'false_positive',
  'false_negative'
] as const;

export type FeedbackEventKind = (typeof FEEDBACK_EVENT_KINDS)[number];

export function isFeedbackEventKind(value: unknown): value is FeedbackEventKind {
  return typeof value === 'string' && (FEEDBACK_EVENT_KINDS as readonly string[]).includes(value);
}

/* ---------------------------------------------------------------------- */
/* Legacy → generic compatibility mapping (Q3.A-modified founder lock)    */
/* ---------------------------------------------------------------------- */

// Maps a legacy email-shaped kind to its Brevio-wide generic (verb, overlay)
// shape. `stop` is INTENTIONALLY NOT MAPPED — per founder lock 2026-06-06:
// "consent/control stays separate from preference learning." STOP/START
// signaling continues to live in memory_signals.stop_active (v0.5.5
// substrate), reply-parser stays unchanged.
//
// The mapping is for AUDIT/CONSUMER ENRICHMENT, NOT for storage normalization.
// The legacy kind continues to be stored literally in feedback_events.kind so
// existing read paths (countByKind('founder_approved'), kernel test's
// countByKind assertions) keep working byte-for-byte. The verb/overlay shape
// flows into:
//   * the `feedback.written` audit detail (verb, role, dimension, legacy_kind)
//   * the applyFeedback consumer's match arm (verb='ignored' + dimension='sender'
//     fires the v0.5.9 sender_feedback_ignored upsert path)
export interface LegacyMappedFeedback {
  readonly verb: BrevioFeedbackEventKind;
  // Detail overlay applied on top of caller-supplied detail when the audit
  // is emitted (the audit-emitting code or the applyFeedback consumer reads
  // these). Storage keeps the caller's literal detail; the overlay is
  // computed at use sites.
  readonly overlay: {
    readonly role?: 'founder' | 'user';
    // Phase v0.5.10 — extended with 'importance' | 'pattern' for the
    // new positive-signal intents (this_mattered / more_like_this).
    // Additive; existing v0.5.9 callers unchanged.
    readonly dimension?: 'sender' | 'alert' | 'ranker_label' | 'thread' | 'topic' | 'importance' | 'pattern';
    readonly reason?: string;
    readonly previous_label?: 'positive' | 'negative';
  };
}

// Internal lookup table. 10 of 11 legacy kinds are mapped here; `stop` is
// deliberately omitted per founder lock. `mapLegacyFeedbackKind` returns
// `null` for unmappable kinds; callers that pass a non-legacy generic kind
// directly receive `null` too (they don't need a mapping).
const LEGACY_FEEDBACK_KIND_MAP: Partial<Record<FeedbackEventKind, LegacyMappedFeedback>> = Object.freeze({
  founder_approved: { verb: 'approved', overlay: { role: 'founder' } },
  founder_rejected: { verb: 'rejected', overlay: { role: 'founder' } },
  // user_opened maps to 'opened' but 'opened' is not in BREVIO_FEEDBACK_EVENT_KINDS
  // for v0.5.9 (no current caller). If a future phase adds 'opened' to the
  // generic kinds, this entry will activate automatically — until then,
  // mapLegacyFeedbackKind returns it but TS narrows to a non-generic verb;
  // callers that match on verb='opened' will simply no-op (the v0.5.9
  // applyFeedback match arm doesn't care about 'opened').
  // Intentionally NOT mapped to keep the BREVIO_FEEDBACK_EVENT_KINDS strict:
  //   user_opened: { verb: 'opened', overlay: { role: 'user' } },
  user_snoozed: { verb: 'snoozed', overlay: { role: 'user', dimension: 'alert' } },
  user_ignored: { verb: 'ignored', overlay: { role: 'user', dimension: 'alert' } },
  ignored_sender: { verb: 'ignored', overlay: { role: 'user', dimension: 'sender' } },
  asked_why: { verb: 'asked_why', overlay: { role: 'user' } },
  no_response: { verb: 'ignored', overlay: { role: 'user', reason: 'no_response' } },
  false_positive: { verb: 'corrected', overlay: { role: 'founder', dimension: 'ranker_label', previous_label: 'positive' } },
  false_negative: { verb: 'corrected', overlay: { role: 'founder', dimension: 'ranker_label', previous_label: 'negative' } }
  // 'stop' INTENTIONALLY OMITTED per founder lock — consent/control stays
  // separate from preference learning. STOP/START continues to flow through
  // memory_signals.stop_active + reply-parser.
});

// Returns a mapped (verb, overlay) for a legacy kind. Returns null for:
//   - the legacy `stop` kind (consent/control, not preference; preserved)
//   - any kind that is already a Brevio generic verb (no mapping needed)
//   - unknown strings (caller's responsibility to validate)
export function mapLegacyFeedbackKind(kind: string): LegacyMappedFeedback | null {
  return LEGACY_FEEDBACK_KIND_MAP[kind as FeedbackEventKind] ?? null;
}

// Convenience: returns the effective verb for a kind (mapped if legacy,
// or the kind itself if already a Brevio generic verb, or null for
// unmappable like 'stop' or unknown strings). Used by the audit emitter
// and applyFeedback consumer to derive `verb` consistently regardless of
// whether the caller passed a legacy or generic kind.
export function resolveFeedbackVerb(kind: string): BrevioFeedbackEventKind | null {
  if (isBrevioFeedbackEventKind(kind)) return kind;
  const mapped = mapLegacyFeedbackKind(kind);
  return mapped ? mapped.verb : null;
}

/* ---------------------------------------------------------------------- */
/* BrevioFeedbackError — write-gate rejection                              */
/* ---------------------------------------------------------------------- */

// Thrown by FeedbackStore.write when the active-surface gate or the
// known-surface gate rejects the write. Callers (slack-interactivity,
// ops-feedback-inject CLI, dispatch executor) catch this to emit a
// `feedback.written` audit row with `result='failure'` + sanitized
// rejection_reason detail. NEVER includes the raw attempted email/payload.
export class BrevioFeedbackError extends Error {
  readonly code: 'unknown_surface' | 'inactive_surface';
  readonly attempted_source_surface: string;
  constructor(code: 'unknown_surface' | 'inactive_surface', attempted_source_surface: string) {
    // Bounded length so a hostile caller can't blow up logs by passing a
    // multi-MB string as `source_surface`.
    const sanitized = String(attempted_source_surface ?? '').slice(0, 64);
    super(`BrevioFeedbackError(${code}): attempted source_surface='${sanitized}'`);
    this.name = 'BrevioFeedbackError';
    this.code = code;
    this.attempted_source_surface = sanitized;
  }
}

/* ---------------------------------------------------------------------- */
/* Event + Input + Store types                                            */
/* ---------------------------------------------------------------------- */

export interface FeedbackEvent {
  readonly id?: number;
  // ISO 8601 timestamp.
  readonly occurred_at: string;
  readonly user_id: string;
  // Some events (stop, no_response system-wide signals) are not tied to a
  // specific alert.
  readonly alert_id: string | null;
  // Set for events about sender behavior (ignored_sender, false_positive
  // tied to a sender). Null when the event has no sender context.
  readonly sender_email: string | null;
  // Stored literal (caller-supplied). Can be a legacy email-shaped kind
  // (e.g. 'founder_approved') OR a Brevio generic verb (e.g. 'ignored').
  // Read paths use `mapLegacyFeedbackKind` / `resolveFeedbackVerb` to
  // surface the generic semantics on top.
  readonly kind: string;
  // Phase v0.5.9 — Brevio-wide surface discriminator. Always set on read
  // (database column is NOT NULL DEFAULT 'email_alert'); existing v0.5.x
  // rows backfilled to 'email_alert' atomically by migration 0007.
  readonly source_surface: BrevioFeedbackSurface;
  // Free-form details, redacted via safe-logger.redact on write.
  readonly detail: Record<string, unknown> | null;
}

export interface FeedbackEventInput {
  user_id: string;
  alert_id: string | null;
  sender_email: string | null;
  // Caller may pass a legacy email-shaped kind OR a Brevio generic verb.
  // The store accepts both; storage is literal (no normalization). The
  // audit emitter and applyFeedback consumer normalize via the mapping
  // helper.
  kind: FeedbackEventKind | BrevioFeedbackEventKind;
  detail?: Record<string, unknown> | null;
  occurred_at?: string;
  // Phase v0.5.9 — optional Brevio-wide surface. Defaults to 'email_alert'
  // when omitted (legacy callers). Writes with a value NOT in
  // BREVIO_FEEDBACK_SURFACES throw BrevioFeedbackError('unknown_surface');
  // writes with a declared-but-inactive value (e.g. 'calendar_reminder' in
  // v0.5.9) throw BrevioFeedbackError('inactive_surface') — this is the
  // load-bearing "not trapped in email" gate proven by smoke C6.
  source_surface?: string;
}

export interface FeedbackStore {
  // Phase v0.5.9 — returns the written event (with id). Existing callers
  // (Slack interactivity, kernel test) historically ignore the return value
  // — that still works. New callers (ops-feedback-inject) use the returned
  // event to pass to applyFeedback for the consumer-side memory_signal
  // upsert.
  write(input: FeedbackEventInput): Promise<FeedbackEvent>;
  recent(userId: string, limit?: number): Promise<readonly FeedbackEvent[]>;
  countByKind(userId: string, kind: FeedbackEventKind | BrevioFeedbackEventKind): Promise<number>;
  countBySender(userId: string, senderEmail: string): Promise<number>;
}

/* ---------------------------------------------------------------------- */
/* Write-gate helper (shared by InMemoryFeedbackStore + PostgresFeedbackStore) */
/* ---------------------------------------------------------------------- */

// Resolves the effective source_surface for a write input + validates it
// against the BREVIO_FEEDBACK_SURFACES enum and BREVIO_FEEDBACK_ACTIVE_SURFACES
// allowlist. Throws BrevioFeedbackError on rejection. Pure function — both
// store impls call it before any side effect.
//
// Privacy invariant: this function does NOT log the attempted value; the
// caller's catch block decides whether/how to surface it (typically via a
// `feedback.written` audit row with `result='failure'`).
export function resolveAndGateSourceSurface(input: FeedbackEventInput): BrevioFeedbackSurface {
  const resolved = input.source_surface ?? 'email_alert';
  if (!isBrevioFeedbackSurface(resolved)) {
    throw new BrevioFeedbackError('unknown_surface', resolved);
  }
  if (!isActiveFeedbackSurface(resolved)) {
    throw new BrevioFeedbackError('inactive_surface', resolved);
  }
  return resolved;
}

/* ---------------------------------------------------------------------- */
/* InMemoryFeedbackStore                                                  */
/* ---------------------------------------------------------------------- */

export class InMemoryFeedbackStore implements FeedbackStore {
  private entries: FeedbackEvent[] = [];
  private nextId = 1;
  private readonly capacity: number;

  constructor(capacity = 10_000) {
    this.capacity = capacity;
  }

  async write(input: FeedbackEventInput): Promise<FeedbackEvent> {
    const source_surface = resolveAndGateSourceSurface(input);
    const detail = input.detail ? (redact(input.detail) as Record<string, unknown>) : null;
    const event: FeedbackEvent = Object.freeze({
      id: this.nextId++,
      occurred_at: input.occurred_at ?? new Date().toISOString(),
      user_id: input.user_id,
      alert_id: input.alert_id,
      sender_email: input.sender_email,
      kind: input.kind,
      source_surface,
      detail
    });
    this.entries.push(event);
    if (this.entries.length > this.capacity) {
      this.entries.splice(0, this.entries.length - this.capacity);
    }
    return event;
  }

  async recent(userId: string, limit = 100): Promise<readonly FeedbackEvent[]> {
    const filtered = this.entries.filter((e) => e.user_id === userId);
    return filtered.slice(-limit).reverse();
  }

  async countByKind(userId: string, kind: FeedbackEventKind | BrevioFeedbackEventKind): Promise<number> {
    return this.entries.filter((e) => e.user_id === userId && e.kind === kind).length;
  }

  async countBySender(userId: string, senderEmail: string): Promise<number> {
    return this.entries.filter((e) => e.user_id === userId && e.sender_email === senderEmail).length;
  }
}
