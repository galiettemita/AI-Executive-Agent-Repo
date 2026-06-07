// Phase v0.5.11 — PIL substrate aggregation pipe.
//
// applyPilAggregation consumes a v0.5.10 reply-parser-routed feedback_event
// (this_mattered / more_like_this / false_positive / ignore_sender) and
// upserts the canonical PIL memory_signal kinds — `sender_importance` and/or
// `sender_suppressed` — keyed by HMAC(sender_email).
//
// Founder-locked invariants (memory: project_v05-11-scope):
//   - HARD #1: live ranker behavior unchanged. This module writes; nothing
//     in the live ranker path reads from it. v0.5.12 gate decides that.
//   - HARD #2: NO raw sender_email in new memory_signal detail or audit
//     detail. Caller passes the pre-HMACed scope_key (alert.sender_email_hash
//     for natural-reply path; computed by ops:feedback-inject for synthetic).
//   - HARD #5: one correction does not flip. `false_positive` lowers
//     sender_importance.score by δ but does NOT flip sender_suppressed.
//     The k-threshold flip only fires when `n_negative_events_after >= k`.
//     `ignore_sender` is the explicit single-event flip carve-out per
//     [[personalized-importance-learning]] §9.1.
//   - HARD #6: recency decay applied at READ time (see pil-context.ts).
//     This module writes raw counters + score; the read side decays.
//   - HARD #7: cross-user contamination — scope_key already encodes user_id
//     via hashSenderKey(); separate (user_id, kind, scope_key) tuples mean
//     User A's signal cannot bind to User B's lookup.
//
// Six-field detail shape on each upsert:
//   sender_importance: { score, n_positive_events, n_negative_events,
//                        last_updated, source_surface, source_feedback_event_ids[] }
//   sender_suppressed: { suppressed:true, set_at, source_surface,
//                        set_by:'explicit_ignore_sender'|'threshold_negative_aggregation',
//                        source_feedback_event_ids[] }
//
// Fifteen-field brevio.signal.aggregated audit detail (per Q6.B): see
// emitSignalAggregatedAudit below.

import { redact } from '../core/safe-logger.js';
import { type AuditStore } from '../core/audit.js';
import { type MemorySignalStore } from './memory-signals.js';

/* ---------------------------------------------------------------------- */
/* Types                                                                  */
/* ---------------------------------------------------------------------- */

// Verb + dimension drive routing. Only the LOCKED four arms cause writes;
// every other (verb, dimension) returns skipped_inactive_dimension.
export type PilVerb = 'approved' | 'corrected' | 'ignored';
export type PilDimension =
  | 'importance' // this_mattered → sender_importance (+δ)
  | 'pattern' // more_like_this → sender_importance (+2δ)
  | 'ranker_label' // false_positive → sender_importance (−δ); may flip sender_suppressed @ k
  | 'sender'; // ignore_sender → sender_suppressed (single-event explicit flip)

export interface PilAggregationInput {
  readonly user_id: string;
  readonly feedback_event_id: number;
  readonly verb: PilVerb;
  readonly dimension: PilDimension;
  readonly source_surface: 'email_alert';
  // HMAC scope_key derived from sender_email by the caller. Null → skip
  // aggregation (the natural-reply path was unable to bind to a sender).
  readonly sender_email_hash: string | null;
  // Tunable thresholds (Q5.C).
  readonly k_threshold: number;
  readonly score_delta: number;
  // Optional clock for tests.
  readonly now?: () => Date;
}

export type PilAggregationOutcome =
  | {
      readonly kind: 'skipped';
      readonly reason:
        | 'no_sender_hash'
        | 'inactive_dimension'
        | 'inactive_source_surface';
    }
  | {
      readonly kind: 'aggregated';
      readonly memory_signal_kind: 'sender_importance' | 'sender_suppressed';
      readonly memory_signal_action: 'created' | 'updated' | 'noop_idempotent';
      readonly scope_key_hash: string;
      readonly score_before: number | null;
      readonly score_after: number | null;
      readonly score_delta: number;
      readonly n_positive_events_before: number;
      readonly n_positive_events_after: number;
      readonly n_negative_events_before: number;
      readonly n_negative_events_after: number;
      readonly suppression_flipped: boolean;
      readonly threshold_k_in_force: number;
    };

export interface PilAggregationDeps {
  readonly memoryStore: MemorySignalStore;
  readonly auditStore: AuditStore;
}

/* ---------------------------------------------------------------------- */
/* Helpers                                                                */
/* ---------------------------------------------------------------------- */

const SOURCE_EVENT_IDS_CAP = 100;

function clampScore(n: number): number {
  if (!Number.isFinite(n)) return 0;
  if (n < -1) return -1;
  if (n > 1) return 1;
  return n;
}

function appendCappedEventId(prior: readonly number[], next: number): number[] {
  if (prior.includes(next)) return [...prior];
  const out = [...prior, next];
  if (out.length > SOURCE_EVENT_IDS_CAP) {
    return out.slice(out.length - SOURCE_EVENT_IDS_CAP);
  }
  return out;
}

function nowIso(input: PilAggregationInput): string {
  return (input.now ? input.now() : new Date()).toISOString();
}

/* ---------------------------------------------------------------------- */
/* applyPilAggregation — main entry                                       */
/* ---------------------------------------------------------------------- */

export async function applyPilAggregation(
  input: PilAggregationInput,
  deps: PilAggregationDeps
): Promise<PilAggregationOutcome> {
  // Hard boundary #9: email_alert only.
  if (input.source_surface !== 'email_alert') {
    return Object.freeze({ kind: 'skipped' as const, reason: 'inactive_source_surface' });
  }
  if (input.sender_email_hash === null) {
    return Object.freeze({ kind: 'skipped' as const, reason: 'no_sender_hash' });
  }

  // Route verb+dimension to the canonical PIL signal arm.
  const arm = resolveArm(input.verb, input.dimension);
  if (arm === null) {
    return Object.freeze({ kind: 'skipped' as const, reason: 'inactive_dimension' });
  }

  const writtenAt = nowIso(input);
  const scope_key = input.sender_email_hash;

  switch (arm) {
    case 'sender_importance_positive':
      return upsertImportance(
        input,
        deps,
        scope_key,
        writtenAt,
        // this_mattered = +δ; more_like_this = +2δ.
        input.dimension === 'pattern' ? input.score_delta * 2 : input.score_delta,
        /* isNegative= */ false
      );
    case 'sender_importance_negative_with_k_check':
      return upsertImportanceNegativeWithKCheck(input, deps, scope_key, writtenAt);
    case 'sender_suppressed_explicit':
      return upsertSuppressedExplicit(input, deps, scope_key, writtenAt);
  }
}

type Arm =
  | 'sender_importance_positive'
  | 'sender_importance_negative_with_k_check'
  | 'sender_suppressed_explicit';

function resolveArm(verb: PilVerb, dimension: PilDimension): Arm | null {
  if (verb === 'approved' && dimension === 'importance') return 'sender_importance_positive';
  if (verb === 'approved' && dimension === 'pattern') return 'sender_importance_positive';
  if (verb === 'corrected' && dimension === 'ranker_label')
    return 'sender_importance_negative_with_k_check';
  if (verb === 'ignored' && dimension === 'sender') return 'sender_suppressed_explicit';
  return null;
}

/* ---------------------------------------------------------------------- */
/* Importance — positive arm (this_mattered / more_like_this)             */
/* ---------------------------------------------------------------------- */

async function upsertImportance(
  input: PilAggregationInput,
  deps: PilAggregationDeps,
  scope_key: string,
  writtenAt: string,
  delta: number,
  isNegative: boolean
): Promise<PilAggregationOutcome> {
  const prior = await deps.memoryStore.get(input.user_id, 'sender_importance', scope_key);
  const priorDetail = (prior?.detail ?? null) as
    | {
        score?: number;
        n_positive_events?: number;
        n_negative_events?: number;
        source_feedback_event_ids?: number[];
      }
    | null;
  const score_before = priorDetail?.score ?? null;
  const n_positive_events_before = priorDetail?.n_positive_events ?? 0;
  const n_negative_events_before = priorDetail?.n_negative_events ?? 0;
  const n_positive_events_after = isNegative
    ? n_positive_events_before
    : n_positive_events_before + 1;
  const n_negative_events_after = isNegative
    ? n_negative_events_before + 1
    : n_negative_events_before;
  const score_after = clampScore((score_before ?? 0) + delta);
  const action: 'created' | 'updated' | 'noop_idempotent' = prior === null ? 'created' : 'updated';

  const priorIds = priorDetail?.source_feedback_event_ids ?? [];
  const nextIds = appendCappedEventId(priorIds, input.feedback_event_id);

  await deps.memoryStore.upsert({
    user_id: input.user_id,
    kind: 'sender_importance',
    scope_key,
    detail: redact({
      score: score_after,
      n_positive_events: n_positive_events_after,
      n_negative_events: n_negative_events_after,
      last_updated: writtenAt,
      source_surface: input.source_surface,
      source_feedback_event_ids: nextIds
    }) as Record<string, unknown>,
    source: 'user_confirmed',
    confidence: 1.0,
    updated_at: writtenAt
  });

  await emitSignalAggregatedAudit(deps, input, {
    memory_signal_kind: 'sender_importance',
    memory_signal_action: action,
    scope_key_hash: scope_key,
    score_before,
    score_after,
    score_delta: delta,
    n_positive_events_before,
    n_positive_events_after,
    n_negative_events_before,
    n_negative_events_after,
    suppression_flipped: false,
    threshold_k_in_force: input.k_threshold
  });

  return Object.freeze({
    kind: 'aggregated' as const,
    memory_signal_kind: 'sender_importance',
    memory_signal_action: action,
    scope_key_hash: scope_key,
    score_before,
    score_after,
    score_delta: delta,
    n_positive_events_before,
    n_positive_events_after,
    n_negative_events_before,
    n_negative_events_after,
    suppression_flipped: false,
    threshold_k_in_force: input.k_threshold
  });
}

/* ---------------------------------------------------------------------- */
/* Importance — negative arm with k-threshold flip                        */
/* ---------------------------------------------------------------------- */

async function upsertImportanceNegativeWithKCheck(
  input: PilAggregationInput,
  deps: PilAggregationDeps,
  scope_key: string,
  writtenAt: string
): Promise<PilAggregationOutcome> {
  // First lower the importance score with the standard −δ shift. This
  // returns the canonical importance outcome — the k-threshold flip is a
  // SECOND write (separate audit row) so the two effects stay legible.
  const importanceOutcome = await upsertImportance(
    input,
    deps,
    scope_key,
    writtenAt,
    -input.score_delta,
    /* isNegative= */ true
  );

  // Pure-narrowing typeguard: importanceOutcome is always `aggregated` for
  // this arm because we already passed the skip checks above; but TS doesn't
  // know that. Treat unexpected shape as a hard failure-of-invariant.
  if (importanceOutcome.kind !== 'aggregated') {
    return importanceOutcome;
  }

  // Now check the k-threshold. ONLY flip when:
  //   1. n_negative_events_after >= k (per founder rule + doctrine §9.1)
  //   2. AND no existing sender_suppressed row exists (don't re-flip already-
  //      suppressed; that's idempotent + saves an audit)
  if (importanceOutcome.n_negative_events_after < input.k_threshold) {
    return importanceOutcome;
  }

  const priorSuppressed = await deps.memoryStore.get(
    input.user_id,
    'sender_suppressed',
    scope_key
  );
  if (priorSuppressed !== null) {
    // Idempotent — already suppressed. Don't emit a redundant audit row.
    return importanceOutcome;
  }

  await deps.memoryStore.upsert({
    user_id: input.user_id,
    kind: 'sender_suppressed',
    scope_key,
    detail: redact({
      suppressed: true,
      set_at: writtenAt,
      source_surface: input.source_surface,
      set_by: 'threshold_negative_aggregation',
      source_feedback_event_ids: [input.feedback_event_id]
    }) as Record<string, unknown>,
    source: 'user_confirmed',
    confidence: 1.0,
    updated_at: writtenAt
  });

  await emitSignalAggregatedAudit(deps, input, {
    memory_signal_kind: 'sender_suppressed',
    memory_signal_action: 'created',
    scope_key_hash: scope_key,
    score_before: importanceOutcome.score_after,
    score_after: importanceOutcome.score_after,
    score_delta: 0,
    n_positive_events_before: importanceOutcome.n_positive_events_after,
    n_positive_events_after: importanceOutcome.n_positive_events_after,
    n_negative_events_before: importanceOutcome.n_negative_events_after,
    n_negative_events_after: importanceOutcome.n_negative_events_after,
    suppression_flipped: true,
    threshold_k_in_force: input.k_threshold
  });

  return Object.freeze({
    kind: 'aggregated' as const,
    memory_signal_kind: 'sender_suppressed',
    memory_signal_action: 'created',
    scope_key_hash: scope_key,
    score_before: importanceOutcome.score_after,
    score_after: importanceOutcome.score_after,
    score_delta: 0,
    n_positive_events_before: importanceOutcome.n_positive_events_after,
    n_positive_events_after: importanceOutcome.n_positive_events_after,
    n_negative_events_before: importanceOutcome.n_negative_events_after,
    n_negative_events_after: importanceOutcome.n_negative_events_after,
    suppression_flipped: true,
    threshold_k_in_force: input.k_threshold
  });
}

/* ---------------------------------------------------------------------- */
/* Suppressed — explicit ignore_sender arm (single-event flip carve-out)  */
/* ---------------------------------------------------------------------- */

async function upsertSuppressedExplicit(
  input: PilAggregationInput,
  deps: PilAggregationDeps,
  scope_key: string,
  writtenAt: string
): Promise<PilAggregationOutcome> {
  const prior = await deps.memoryStore.get(input.user_id, 'sender_suppressed', scope_key);
  const priorDetail = (prior?.detail ?? null) as
    | { source_feedback_event_ids?: number[]; set_by?: string }
    | null;
  const action: 'created' | 'updated' | 'noop_idempotent' = prior === null ? 'created' : 'updated';
  const suppression_flipped = prior === null;
  const priorIds = priorDetail?.source_feedback_event_ids ?? [];
  const nextIds = appendCappedEventId(priorIds, input.feedback_event_id);

  await deps.memoryStore.upsert({
    user_id: input.user_id,
    kind: 'sender_suppressed',
    scope_key,
    detail: redact({
      suppressed: true,
      set_at: writtenAt,
      source_surface: input.source_surface,
      set_by: 'explicit_ignore_sender',
      source_feedback_event_ids: nextIds
    }) as Record<string, unknown>,
    source: 'user_confirmed',
    confidence: 1.0,
    updated_at: writtenAt
  });

  await emitSignalAggregatedAudit(deps, input, {
    memory_signal_kind: 'sender_suppressed',
    memory_signal_action: action,
    scope_key_hash: scope_key,
    score_before: null,
    score_after: null,
    score_delta: 0,
    n_positive_events_before: 0,
    n_positive_events_after: 0,
    n_negative_events_before: 0,
    n_negative_events_after: 0,
    suppression_flipped,
    threshold_k_in_force: input.k_threshold
  });

  return Object.freeze({
    kind: 'aggregated' as const,
    memory_signal_kind: 'sender_suppressed',
    memory_signal_action: action,
    scope_key_hash: scope_key,
    score_before: null,
    score_after: null,
    score_delta: 0,
    n_positive_events_before: 0,
    n_positive_events_after: 0,
    n_negative_events_before: 0,
    n_negative_events_after: 0,
    suppression_flipped,
    threshold_k_in_force: input.k_threshold
  });
}

/* ---------------------------------------------------------------------- */
/* Audit emitter — 15 locked Q6.B detail fields                           */
/* ---------------------------------------------------------------------- */

interface AuditFields {
  readonly memory_signal_kind: 'sender_importance' | 'sender_suppressed';
  readonly memory_signal_action: 'created' | 'updated' | 'noop_idempotent';
  readonly scope_key_hash: string;
  readonly score_before: number | null;
  readonly score_after: number | null;
  readonly score_delta: number;
  readonly n_positive_events_before: number;
  readonly n_positive_events_after: number;
  readonly n_negative_events_before: number;
  readonly n_negative_events_after: number;
  readonly suppression_flipped: boolean;
  readonly threshold_k_in_force: number;
}

async function emitSignalAggregatedAudit(
  deps: PilAggregationDeps,
  input: PilAggregationInput,
  fields: AuditFields
): Promise<void> {
  await deps.auditStore.write({
    actor_user_id: input.user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'brevio.signal.aggregated',
    target: `memory_signal:${fields.memory_signal_kind}`,
    result: 'success',
    detail: {
      verb: input.verb,
      dimension: input.dimension,
      feedback_event_id: input.feedback_event_id,
      source_surface: input.source_surface,
      memory_signal_kind: fields.memory_signal_kind,
      memory_signal_action: fields.memory_signal_action,
      memory_signal_scope_key_hash: fields.scope_key_hash,
      score_before: fields.score_before,
      score_after: fields.score_after,
      score_delta: fields.score_delta,
      n_positive_events_before: fields.n_positive_events_before,
      n_positive_events_after: fields.n_positive_events_after,
      n_negative_events_before: fields.n_negative_events_before,
      n_negative_events_after: fields.n_negative_events_after,
      suppression_flipped: fields.suppression_flipped,
      threshold_k_in_force: fields.threshold_k_in_force
    }
  });
}

/* ---------------------------------------------------------------------- */
/* Env loader for Q5.C tunables                                           */
/* ---------------------------------------------------------------------- */

export interface PilTunables {
  readonly k_threshold: number;
  readonly score_delta: number;
  readonly recency_full_days: number;
  readonly recency_decay_days: number;
}

const DEFAULT_TUNABLES: PilTunables = Object.freeze({
  k_threshold: 3,
  score_delta: 0.1,
  recency_full_days: 90,
  recency_decay_days: 90
});

export function loadPilTunables(env: Record<string, string | undefined>): PilTunables {
  const parseInt = (name: string, fallback: number, min: number, max: number | null): number => {
    const raw = (env[name] ?? '').trim();
    if (!raw) return fallback;
    const n = Number(raw);
    if (!Number.isFinite(n) || !Number.isInteger(n) || n < min || (max !== null && n > max)) {
      throw new Error(`${name}='${raw}' must be an integer in [${min}, ${max ?? '∞'}]`);
    }
    return n;
  };
  const parseFloat = (
    name: string,
    fallback: number,
    minExclusive: number,
    maxInclusive: number
  ): number => {
    const raw = (env[name] ?? '').trim();
    if (!raw) return fallback;
    const n = Number(raw);
    if (!Number.isFinite(n) || n <= minExclusive || n > maxInclusive) {
      throw new Error(
        `${name}='${raw}' must be a finite number in (${minExclusive}, ${maxInclusive}]`
      );
    }
    return n;
  };
  return Object.freeze({
    k_threshold: parseInt('FOMO_PIL_K_THRESHOLD', DEFAULT_TUNABLES.k_threshold, 1, null),
    score_delta: parseFloat('FOMO_PIL_SCORE_DELTA', DEFAULT_TUNABLES.score_delta, 0, 0.5),
    recency_full_days: parseInt(
      'FOMO_PIL_RECENCY_FULL_DAYS',
      DEFAULT_TUNABLES.recency_full_days,
      1,
      null
    ),
    recency_decay_days: parseInt(
      'FOMO_PIL_RECENCY_DECAY_DAYS',
      DEFAULT_TUNABLES.recency_decay_days,
      0,
      null
    )
  });
}

export const PIL_DEFAULT_TUNABLES = DEFAULT_TUNABLES;
