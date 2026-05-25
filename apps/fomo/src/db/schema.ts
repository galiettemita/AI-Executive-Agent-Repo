// Drizzle schema for the v0.1 persistence substrate.
//
// Phase 2E shipped the SUBSTRATE tables. Workflow tables land in Phase 3
// as their callers do. Current state:
//   * gmail_cursors    — Phase 3B.1 (caller: Gmail polling worker)
//   * rank_results     — Phase 3C.3 (caller: Gmail polling worker → ranker)
//   * alerts           — Phase 3D.1 (caller: Slack candidate review posting)
//   * message_events / replies / sender_importance / suppressions /
//     user_preferences — still deferred; land with their callers.
// No fake/empty tables here.
//
// Design choices:
//   * No foreign keys — keeps Phase 2E tests free of insertion-order
//     constraints and matches the in-memory stores' loose-coupling shape.
//     FKs can be added in Phase 3 with their callers if useful.
//   * timestamps use `timestamp with time zone` (timestamptz) so ISO 8601
//     strings round-trip cleanly through JS Date.
//   * jsonb for free-form detail/metadata so the store layer can hand the
//     redacted records to Postgres as-is.
//   * serial primary keys for log-style tables (audit_log, feedback_events,
//     cost_records, etc.) match the in-memory stores' nextId pattern.
//   * memory_signals has a unique index on (user_id, kind, scope_key) so
//     upserts use ON CONFLICT.

import {
  bigint,
  bigserial,
  boolean,
  doublePrecision,
  index,
  integer,
  jsonb,
  pgTable,
  primaryKey,
  text,
  timestamp,
  uniqueIndex,
  uuid
} from 'drizzle-orm/pg-core';

/* ---------------------------------------------------------------------- */
/* users                                                                  */
/* ---------------------------------------------------------------------- */

export const users = pgTable(
  'users',
  {
    id: uuid('id').primaryKey().defaultRandom(),
    email: text('email').notNull(),
    timezone: text('timezone'),
    created_at: timestamp('created_at', { withTimezone: true }).notNull().defaultNow()
  },
  (table) => [uniqueIndex('users_email_uq').on(table.email)]
);

/* ---------------------------------------------------------------------- */
/* oauth_tokens — mirrors InMemoryTokenStore from Phase 2A                */
/* ---------------------------------------------------------------------- */

export const oauth_tokens = pgTable(
  'oauth_tokens',
  {
    user_id: text('user_id').notNull(),
    provider: text('provider').notNull(),
    scopes: jsonb('scopes').notNull().$type<string[]>(),
    access_token_ciphertext: text('access_token_ciphertext').notNull(), // base64 of bytea
    refresh_token_ciphertext: text('refresh_token_ciphertext'),
    expires_at: timestamp('expires_at', { withTimezone: true }),
    obtained_at: timestamp('obtained_at', { withTimezone: true }).notNull().defaultNow(),
    last_refreshed_at: timestamp('last_refreshed_at', { withTimezone: true }),
    needs_reauth: boolean('needs_reauth').notNull().default(false),
    key_version: integer('key_version').notNull().default(1)
  },
  (table) => [primaryKey({ columns: [table.user_id, table.provider] })]
);

/* ---------------------------------------------------------------------- */
/* consent — per-user, per-tool grant log                                 */
/* ---------------------------------------------------------------------- */

export const consent = pgTable(
  'consent',
  {
    user_id: text('user_id').notNull(),
    tool_id: text('tool_id').notNull(),
    granted_at: timestamp('granted_at', { withTimezone: true }).notNull().defaultNow(),
    revoked_at: timestamp('revoked_at', { withTimezone: true })
  },
  (table) => [primaryKey({ columns: [table.user_id, table.tool_id] })]
);

/* ---------------------------------------------------------------------- */
/* audit_log — mirrors InMemoryAuditStore from Phase 2A                   */
/* ---------------------------------------------------------------------- */

export const audit_log = pgTable(
  'audit_log',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    occurred_at: timestamp('occurred_at', { withTimezone: true }).notNull().defaultNow(),
    actor_user_id: text('actor_user_id'),
    actor_ip: text('actor_ip'),
    actor_user_agent: text('actor_user_agent'),
    action: text('action').notNull(),
    target: text('target'),
    result: text('result').notNull(),
    detail: jsonb('detail').$type<Record<string, unknown> | null>()
  },
  (table) => [
    index('audit_log_actor_user_id_idx').on(table.actor_user_id),
    index('audit_log_occurred_at_idx').on(table.occurred_at)
  ]
);

/* ---------------------------------------------------------------------- */
/* feedback_events — mirrors InMemoryFeedbackStore from Phase 2C          */
/* ---------------------------------------------------------------------- */

export const feedback_events = pgTable(
  'feedback_events',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    occurred_at: timestamp('occurred_at', { withTimezone: true }).notNull().defaultNow(),
    user_id: text('user_id').notNull(),
    alert_id: text('alert_id'),
    sender_email: text('sender_email'),
    kind: text('kind').notNull(),
    detail: jsonb('detail').$type<Record<string, unknown> | null>()
  },
  (table) => [
    index('feedback_events_user_id_idx').on(table.user_id),
    index('feedback_events_kind_idx').on(table.user_id, table.kind),
    index('feedback_events_sender_idx').on(table.user_id, table.sender_email)
  ]
);

/* ---------------------------------------------------------------------- */
/* memory_signals — mirrors InMemoryMemorySignalStore from Phase 2C       */
/* ---------------------------------------------------------------------- */
//
// Identity is (user_id, kind, scope_key); the Postgres store uses an
// ON CONFLICT (user_id, kind, scope_key) DO UPDATE upsert. The unique
// index below makes that conflict target real.
//
// scope_key is nullable, so the unique index uses COALESCE(scope_key, '')
// via a generated column would be ideal but Drizzle doesn't expose that
// cleanly across PG versions. We instead persist scope_key='' for the
// null case in the Postgres store layer.

export const memory_signals = pgTable(
  'memory_signals',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    updated_at: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
    user_id: text('user_id').notNull(),
    kind: text('kind').notNull(),
    // Empty string sentinel for the "no scope" case (the store maps null↔'').
    scope_key: text('scope_key').notNull().default(''),
    detail: jsonb('detail').notNull().$type<Record<string, unknown>>(),
    confidence: doublePrecision('confidence').notNull(),
    source: text('source').notNull()
  },
  (table) => [
    uniqueIndex('memory_signals_identity_uq').on(table.user_id, table.kind, table.scope_key)
  ]
);

/* ---------------------------------------------------------------------- */
/* alert_state_transitions — new in Phase 2E                              */
/* ---------------------------------------------------------------------- */

export const alert_state_transitions = pgTable(
  'alert_state_transitions',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    alert_id: text('alert_id').notNull(),
    user_id: text('user_id').notNull(),
    from_state: text('from_state').notNull(),
    to_state: text('to_state').notNull(),
    at: timestamp('at', { withTimezone: true }).notNull().defaultNow(),
    reason: text('reason').notNull()
  },
  (table) => [
    index('alert_state_transitions_alert_idx').on(table.alert_id),
    index('alert_state_transitions_user_idx').on(table.user_id, table.at)
  ]
);

/* ---------------------------------------------------------------------- */
/* cost_records — mirrors InMemoryCostStore from Phase 2D                 */
/* ---------------------------------------------------------------------- */

export const cost_records = pgTable(
  'cost_records',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    occurred_at: timestamp('occurred_at', { withTimezone: true }).notNull().defaultNow(),
    user_id: text('user_id').notNull(),
    capability: text('capability').notNull(),
    model_name: text('model_name').notNull(),
    prompt_version: text('prompt_version').notNull(),
    latency_ms: integer('latency_ms').notNull(),
    input_tokens: integer('input_tokens').notNull(),
    output_tokens: integer('output_tokens').notNull(),
    // doublePrecision is fine for v0.1 cost arithmetic; numeric(12,6) is
    // overkill for the per-call dollar amounts we expect.
    estimated_cost_usd: doublePrecision('estimated_cost_usd').notNull(),
    schema_valid: boolean('schema_valid').notNull()
  },
  (table) => [
    index('cost_records_user_idx').on(table.user_id, table.occurred_at),
    index('cost_records_model_idx').on(table.user_id, table.model_name)
  ]
);

/* ---------------------------------------------------------------------- */
/* tool_invocations — new in Phase 2E per FOMO_PLAN §9.10                 */
/* ---------------------------------------------------------------------- */
//
// Privacy invariant: no raw payload content. Only operational metadata.
// The Postgres store mirrors the in-memory store's safe-logger redact()
// pass on the metadata jsonb column.

export const tool_invocations = pgTable(
  'tool_invocations',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    occurred_at: timestamp('occurred_at', { withTimezone: true }).notNull().defaultNow(),
    user_id: text('user_id').notNull(),
    tool_id: text('tool_id').notNull(),
    invocation_id: text('invocation_id').notNull(),
    policy_decision: text('policy_decision').notNull(),
    status: text('status').notNull(),
    latency_ms: integer('latency_ms'),
    error_code: text('error_code'),
    error_reason: text('error_reason'),
    metadata: jsonb('metadata').$type<Record<string, unknown> | null>()
  },
  (table) => [
    uniqueIndex('tool_invocations_invocation_id_uq').on(table.invocation_id),
    index('tool_invocations_user_idx').on(table.user_id, table.occurred_at),
    index('tool_invocations_tool_idx').on(table.user_id, table.tool_id),
    index('tool_invocations_status_idx').on(table.user_id, table.status)
  ]
);

/* ---------------------------------------------------------------------- */
/* gmail_cursors — new in Phase 3B.1                                      */
/* ---------------------------------------------------------------------- */
//
// Per-user Gmail history_id cursor for incremental polling. Identity is
// user_id (one cursor per user — Phase 3B.2's polling worker reads and
// advances it). history_id is Gmail's opaque uint64; stored as text to
// avoid JS number precision pitfalls.

export const gmail_cursors = pgTable('gmail_cursors', {
  user_id: text('user_id').primaryKey(),
  history_id: text('history_id').notNull(),
  updated_at: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow()
});

/* ---------------------------------------------------------------------- */
/* rank_results — new in Phase 3C.3                                       */
/* ---------------------------------------------------------------------- */
//
// One row per successful ranker invocation. The polling worker reads a
// Gmail message via gmail.read (audit + tool_invocations capture the
// read), then hands the RawEmailContext to the ranker. On RankerSuccess
// a row lands here; on RankerFailure only an audit event is written
// (this table is "successful ranks only" — easier to query for the 3D
// Slack founder review cards and for the 3C.4 evidence script).
//
// Privacy invariants:
//   * NO body content. The ranker prompt sees only the egress-redacted
//     view (sender, subject, ≤240-char snippet, attachment counts) and
//     this table stores the model's DECISION, not its prompt input.
//   * `reason` is the model-authored RankDecision.reason field, bounded
//     to MAX_REASON_LEN (240) by the validator. It is summary text,
//     not body text — the leak-canary scan still treats long strings
//     as suspicious.
//
// Idempotency: unique on (user_id, message_id). The store's write path
// uses ON CONFLICT DO NOTHING and reports `inserted: false` so the
// worker can audit `fomo.rank.already_ranked` instead of double-charging
// model credits when Gmail history replays a message_id.

export const rank_results = pgTable(
  'rank_results',
  {
    id: bigserial('id', { mode: 'number' }).primaryKey(),
    user_id: text('user_id').notNull(),
    message_id: text('message_id').notNull(),
    invocation_id: text('invocation_id').notNull(),
    model_name: text('model_name').notNull(),
    prompt_version: text('prompt_version').notNull(),
    label: text('label').notNull(),
    score: doublePrecision('score').notNull(),
    reason: text('reason').notNull(),
    latency_ms: integer('latency_ms').notNull(),
    input_tokens: integer('input_tokens').notNull(),
    output_tokens: integer('output_tokens').notNull(),
    estimated_cost_usd: doublePrecision('estimated_cost_usd').notNull(),
    created_at: timestamp('created_at', { withTimezone: true }).notNull().defaultNow()
  },
  (table) => [
    uniqueIndex('rank_results_user_message_uq').on(table.user_id, table.message_id),
    index('rank_results_user_created_idx').on(table.user_id, table.created_at),
    index('rank_results_label_idx').on(table.user_id, table.label)
  ]
);

/* ---------------------------------------------------------------------- */
/* alerts — new in Phase 3D.1                                             */
/* ---------------------------------------------------------------------- */
//
// One row per candidate alert posted (or to be posted) to the founder
// Slack channel for review. Created when:
//   1. ranker returned a RankerSuccess and wrote a rank_results row, AND
//   2. that rank_results row has label='important', AND
//   3. FOMO_SLACK_REVIEW_ENABLED is on
//
// State lives in alert_state_transitions (transition: ranked →
// queued_for_review on Slack post success; ranked → failed on post
// failure). 3D.1 does NOT add the approval-receiving path; alerts sit
// at queued_for_review indefinitely until Phase 3D.2 wires the
// inbound Slack callback that transitions queued_for_review →
// approved | rejected.
//
// Idempotency: UNIQUE on rank_result_id. A given rank can produce AT
// MOST one alert. When the polling worker re-observes a message that
// is already ranked (fomo.rank.already_ranked path), the alert
// creation MUST find the existing row instead of inserting — the
// store returns { inserted: false } and the worker audits
// `fomo.slack.already_alerted` instead of posting a second Slack card.
// This protects the founder's Slack channel from re-spamming on
// cursor rewinds, Gmail history replays, or worker restarts.
//
// Privacy: alerts table holds ONLY operational identifiers (alert_id,
// user_id, message_id, rank_result_id, label, score, created_at).
// NO body content. NO subject line. NO sender_email. The Slack card
// itself goes through applyEgressForSlackCard before posting; the
// payload sent to Slack is never persisted here.

export const alerts = pgTable(
  'alerts',
  {
    alert_id: text('alert_id').primaryKey(),
    user_id: text('user_id').notNull(),
    message_id: text('message_id').notNull(),
    rank_result_id: bigint('rank_result_id', { mode: 'number' }).notNull(),
    label: text('label').notNull(),
    score: doublePrecision('score').notNull(),
    created_at: timestamp('created_at', { withTimezone: true }).notNull().defaultNow()
  },
  (table) => [
    uniqueIndex('alerts_rank_result_uq').on(table.rank_result_id),
    index('alerts_user_created_idx').on(table.user_id, table.created_at),
    index('alerts_user_message_idx').on(table.user_id, table.message_id)
  ]
);
