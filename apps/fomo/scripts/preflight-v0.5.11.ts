// Phase v0.5.11 preflight — PIL substrate + shadow ranker context/eval (founder-only smoke).
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-07):
//   This script is part of the v0.5.11 SCAFFOLDING commit, which lands BEFORE
//   the runtime implementation. v0.5.11 turns v0.5.9/v0.5.10 feedback into
//   per-user importance memory signals AND proves how those signals would be
//   presented to the ranker in SHADOW/EVAL mode, without changing live ranker
//   behavior.
//
//   Locked scope (see memory project_v05-11-scope):
//     Q1: modified hybrid — aggregation substrate + sender_email threading +
//         shadow ranker context/eval contract; NO live ranker change.
//     Q2.B: two new memory_signal kinds — `sender_importance`,
//           `sender_suppressed`. v0.5.9 `sender_feedback_ignored` UNTOUCHED
//           (no migration this phase).
//     Q3.B: linear recency decay — full weight 0–90d, linear 90–180d, zero
//           after 180d; computed at READ time inside buildPilContext().
//     Q4.A: include sender_email threading — new alerts.sender_email_hash
//           column (HMAC); closes v0.5.10 §15 bonus finding #1.
//     Q5.C: tunable env — FOMO_PIL_K_THRESHOLD (default 3),
//           FOMO_PIL_SCORE_DELTA (default 0.1),
//           FOMO_PIL_RECENCY_FULL_DAYS (default 90),
//           FOMO_PIL_RECENCY_DECAY_DAYS (default 90).
//     Q6.B: new audit kind `brevio.signal.aggregated` with 15 locked detail
//           fields.
//
//   The runtime commit(s) will:
//     - Add migration 0008_alerts_sender_email_hash.sql (HMAC column on alerts).
//     - Add migration 0009 (or code-level enum extension) to register
//       `sender_importance` + `sender_suppressed` in MEMORY_SIGNAL_KINDS.
//     - Wire `apps/fomo/src/memory/pil-aggregation.ts` — applyPilAggregation()
//       consumer that listens for this_mattered / more_like_this /
//       false_positive / ignore_sender feedback_events and upserts the two
//       new kinds.
//     - Wire `apps/fomo/src/ranker/pil-context.ts` — buildPilContext()
//       projection module computing decayed score at READ time.
//     - Extend the ranker prompt builder with OPTIONAL `pil_context` field.
//       Production call site passes `pil_context: null` UNCONDITIONALLY in
//       v0.5.11 (LOCKED — no env switch flips it on).
//     - Add `apps/fomo/src/eval/pil-shadow.eval.ts` — offline shadow eval
//       harness with ≥10 synthetic fixtures asserting shift DIRECTION.
//     - Register `brevio.signal.aggregated` in FOMO_AUDIT_ACTIONS with 15
//       locked detail fields.
//     - Thread HMAC(sender_email) at rank step into the new
//       alerts.sender_email_hash column.
//
//   While the runtime commit is pending:
//     - migration 0008 not applied → operator reminder (DB shape)
//     - pil-aggregation.ts module not present → WARN PENDING (dynamic probe)
//     - pil-context.ts module not present → WARN PENDING (dynamic probe)
//     - shadow eval script not present → WARN PENDING
//     - brevio.signal.aggregated not in FOMO_AUDIT_ACTIONS → WARN PENDING
//     - sender_importance / sender_suppressed missing from MEMORY_SIGNAL_KINDS
//       → ERROR (they are pre-existing as of v0.5.10; missing = regression)
//
//   When runtime + smoke + reports land, all WARNs flip to silent.
//
// Pure config inspection — no DB, no network. Validates that:
//   (a) the substrate is the v0.5.10-PASS shape (so v0.5.11 layers on a
//       known-good base)
//   (b) the v0.5.11-specific founder gate is set
//       (FOMO_V0_5_11_BASELINE_CONFIRMED)
//   (c) the four new tunable env vars parse + are within bounds
//   (d) carry-forward invariants (HMR template, BREVIO_FEEDBACK surfaces,
//       sender_feedback_ignored UNTOUCHED, reply-parser-v0.2.0) still hold
//
// Forbidden in v0.5.11 (preserves prior phase locks + permanent product
// principles):
//   * Live ranker behavior change — production ranker call site MUST pass
//     `pil_context: null` unconditionally; no env switch flips it on this
//     phase
//   * Ranker prompt version bump for production (ranker-v0.2.0 unchanged)
//   * Auto-suppression in live alerts (sender_suppressed is write-only WRT
//     the dispatch path this phase)
//   * HMR Feedback Acknowledgment / new user-facing copy (Q5.A defer still
//     holds from v0.5.10)
//   * New `source_surface` beyond email_alert
//   * Auto-send (own gate per FOMO_PLAN v0.8)
//   * FOMO_AUTO_SEND_ENABLED=true
//   * BREVIO_DEV_MODE=true
//   * Migration / rename of v0.5.9 sender_feedback_ignored (own future
//     hardening phase)
//   * Cross-user aggregation (load-bearing privacy invariant)
//   * Raw subject / body / snippet / sender_email in new memory_signal or
//     audit detail
//   * Global learning from one user (per [[personalized-importance-learning]]
//     §9.10)
//   * Single correction flips sender behavior — only explicit ignore_sender
//     OR n_negative_events ≥ k flips sender_suppressed
//   * Storing reply text in any column / detail field
//   * STOP/START as preference feedback (deterministic compliance unchanged)
//   * 3E.1 reversal (no LLM in body composition; ranker still produces
//     rank.reason only)
//   * Friend C onboarding (three-friend cap)
//
// The runbook (docs/smoke-test-v0.5.11-pil-substrate-shadow.md) covers
// out-of-band requirements preflight cannot check:
//   * Baseline snapshot of recent feedback_events + sender_feedback_ignored
//     + sender_importance/sender_suppressed (expected 0 before smoke) +
//     brevio.signal.aggregated audit rows (expected 0 before smoke)
//     captured (so post-smoke diff proves new rows are from the smoke)
//   * Migration 0008 (alerts.sender_email_hash) applied to live Neon BEFORE
//     boot (verified by querying information_schema.columns; documented in
//     §3 of the runbook)
//   * Migration 0009 (MEMORY_SIGNAL_KINDS extension) applied to live Neon
//     (if enum-backed) BEFORE boot
//   * SendBlue inbound webhook reachable (ngrok recommended). Same tier
//     caveats as v0.5.10 apply per [[sendblue-plan-gates]]; signed-curl
//     substitute pattern continues to be acceptable per founder rule
//     2026-06-07 (LOAD-BEARING ignore_sender path used signed-curl in
//     v0.5.10 smoke and is explicitly accepted with the documented note).
//   * No friend involved this phase (three-friend cap; founder-only smoke).

import { loadKillSwitches } from '../src/core/kill-switches.js';
import { FOMO_AUDIT_ACTIONS, type AuditAction } from '../src/core/audit.js';
import { MEMORY_SIGNAL_KINDS } from '../src/memory/memory-signals.js';
import { FOUNDER_TEXT_TEMPLATE_VERSION } from '../src/core/founder-text-template.js';
import {
  BREVIO_FEEDBACK_ACTIVE_SURFACES,
  BREVIO_FEEDBACK_EVENT_KINDS,
  BREVIO_FEEDBACK_SURFACES
} from '../src/memory/feedback-events.js';
import { PROMPT_VERSION as REPLY_PARSER_PROMPT_VERSION } from '../src/reply-parser/prompt.js';

type Severity = 'error' | 'warn';
interface Check {
  readonly name: string;
  readonly severity: Severity;
  readonly message: string;
}
const issues: Check[] = [];

function require_(name: string, message: string): void {
  if (!(process.env[name] ?? '').trim()) issues.push({ name, severity: 'error', message });
}
function requireMin(name: string, minBytes: number, message: string): void {
  const raw = (process.env[name] ?? '').trim();
  if (!raw) {
    issues.push({ name, severity: 'error', message: `${message} (missing)` });
    return;
  }
  let decoded: Buffer;
  try {
    decoded = raw.startsWith('hex:') ? Buffer.from(raw.slice(4), 'hex') : Buffer.from(raw, 'base64');
  } catch {
    issues.push({ name, severity: 'error', message: `${message} — not a valid base64 or hex: value` });
    return;
  }
  if (decoded.length < minBytes) {
    issues.push({
      name,
      severity: 'error',
      message: `${message} — decoded length ${decoded.length} bytes, need ${minBytes}`
    });
  }
}
function expectEquals(name: string, expected: string, message: string): void {
  const v = (process.env[name] ?? '').trim();
  if (v !== expected) {
    issues.push({
      name,
      severity: 'error',
      message: `${message} (expected '${expected}', got '${v || '<unset>'}')`
    });
  }
}
function checkCycleMin(name: string, min: number, ctx: string): void {
  const raw = (process.env[name] ?? '').trim();
  if (!raw) {
    issues.push({
      name,
      severity: 'error',
      message: `${name} required for v0.5.11 (${ctx}). Set ≥ ${min} explicitly.`
    });
    return;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || n < min) {
    issues.push({
      name,
      severity: 'error',
      message: `${name}=${raw} is below the v0.5.11 minimum of ${min}. ${ctx}`
    });
  }
}
function checkIntBounds(
  name: string,
  defaultVal: number,
  min: number,
  max: number | null,
  ctx: string
): void {
  const raw = (process.env[name] ?? '').trim();
  if (!raw) {
    issues.push({
      name,
      severity: 'warn',
      message: `${name} unset; runtime will use default ${defaultVal}. ${ctx}`
    });
    return;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || !Number.isInteger(n) || n < min || (max !== null && n > max)) {
    issues.push({
      name,
      severity: 'error',
      message: `${name}='${raw}' must be an integer in [${min}, ${max ?? '∞'}]. ${ctx}`
    });
  }
}
function checkFloatBounds(
  name: string,
  defaultVal: number,
  min: number,
  max: number,
  ctx: string
): void {
  const raw = (process.env[name] ?? '').trim();
  if (!raw) {
    issues.push({
      name,
      severity: 'warn',
      message: `${name} unset; runtime will use default ${defaultVal}. ${ctx}`
    });
    return;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || n <= min || n > max) {
    issues.push({
      name,
      severity: 'error',
      message: `${name}='${raw}' must be a finite number in (${min}, ${max}]. ${ctx}`
    });
  }
}

console.log('Phase v0.5.11 preflight — PIL substrate + shadow ranker context/eval (founder-only smoke)\n');

/* ---------------------------------------------------------------------- */
/* Substrate carry-forward (v0.5.1 through v0.5.10)                       */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required.');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');
requireMin('BREVIO_PHONE_HASH_KEY', 32, 'BREVIO_PHONE_HASH_KEY must be a 32-byte key');
// LOAD-BEARING for v0.5.11: the new alerts.sender_email_hash column and the
// new memory_signal scope_key both HMAC sender_email with this key. v0.5.9
// already required it; v0.5.11 deepens the load by reading it at rank step.
requireMin(
  'BREVIO_SENDER_HASH_KEY',
  32,
  'BREVIO_SENDER_HASH_KEY must be a 32-byte key. v0.5.11 LOAD-BEARING — used both for the new alerts.sender_email_hash column (rank-step write) AND the sender_importance / sender_suppressed scope_key (aggregation upsert) AND for buildPilContext lookup. Symmetry with v0.5.9 carry-forward.'
);

expectEquals(
  'FOMO_FRIEND_BETA_ENABLED',
  'true',
  'v0.5.11 substrate still requires the friend-beta kill switch ON (carry-forward; no friend involved this phase but substrate stays live).'
);

require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required (the polled-alert chain that produces real iMessage threads for the smoke still uses Gmail).');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required.');
require_('BREVIO_OAUTH_REDIRECT_URI_GOOGLE', 'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required.');

require_('SENDBLUE_API_KEY_ID', 'SENDBLUE_API_KEY_ID required.');
require_('SENDBLUE_API_SECRET_KEY', 'SENDBLUE_API_SECRET_KEY required.');
require_('SENDBLUE_FROM_NUMBER', 'SENDBLUE_FROM_NUMBER required.');
require_('SENDBLUE_WEBHOOK_SECRET', 'SENDBLUE_WEBHOOK_SECRET required (inbound webhook signature gate; used by the route handler AND the signed-curl substitute fallback per v0.5.10 PASS note).');

require_('OPENAI_API_KEY', 'OPENAI_API_KEY required (reply-parser-v0.2.0 classifier + ranker continue to run; shadow eval harness also calls OpenAI).');
expectEquals('FOMO_RANKER_ENABLED', 'true', 'Ranker must be on — the rank-step write path is where sender_email_hash gets threaded onto alerts.');
expectEquals(
  'FOMO_SLACK_REVIEW_ENABLED',
  'true',
  'Slack review must be on — substrate live for the smoke alert produced by the ranker.'
);
expectEquals(
  'FOMO_SEND_ENABLED',
  'true',
  'FOMO_SEND_ENABLED must be true so the outbound worker can produce the alert message the smoke reply is threaded against.'
);
expectEquals(
  'FOMO_GMAIL_POLLING_ENABLED',
  'true',
  'Polling must be on so the alert that the smoke replies to gets surfaced.'
);
expectEquals(
  'FOMO_SENDBLUE_INBOUND_ENABLED',
  'true',
  'FOMO_SENDBLUE_INBOUND_ENABLED must be true — the inbound webhook is the natural-reply ingestion point.'
);
require_('SLACK_BOT_TOKEN', 'SLACK_BOT_TOKEN required.');
require_('SLACK_FOUNDER_CHANNEL_ID', 'SLACK_FOUNDER_CHANNEL_ID required.');
require_('SLACK_SIGNING_SECRET', 'SLACK_SIGNING_SECRET required.');
require_('FOMO_FOUNDER_USER_ID', 'FOMO_FOUNDER_USER_ID required.');

const founderPhone = (process.env.FOMO_FOUNDER_PHONE_NUMBER ?? '').trim();
if (!founderPhone) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message:
      'FOMO_FOUNDER_PHONE_NUMBER required — the SendBlue inbound webhook routes by from-phone, so the founder phone must be registered.'
  });
} else if (/^\+1555010\d{4}$/.test(founderPhone)) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message: `FOMO_FOUNDER_PHONE_NUMBER='+1555010xxxx' is NANPA-reserved fictional. Set a real founder phone.`
  });
}

const friendBaseUrl = (process.env.FOMO_FRIEND_BETA_BASE_URL ?? '').trim();
if (!friendBaseUrl) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_BASE_URL',
    severity: 'error',
    message:
      'FOMO_FRIEND_BETA_BASE_URL required (substrate live). HTTPS URL is also where SendBlue posts inbound webhooks.'
  });
} else if (!/^https:\/\//.test(friendBaseUrl)) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_BASE_URL',
    severity: 'error',
    message: `FOMO_FRIEND_BETA_BASE_URL='${friendBaseUrl.slice(0, 60)}...' must start with https://.`
  });
}

checkCycleMin(
  'FOMO_GMAIL_POLLING_MAX_CYCLES',
  30,
  'v0.5.11 smoke needs the polling worker live long enough for at least one alert to be ranked + delivered so the new sender_email_hash thread is actually exercised on a fresh row.'
);
checkCycleMin(
  'FOMO_OUTBOUND_MAX_CYCLES',
  30,
  'v0.5.11 smoke keeps the outbound worker live so the alert iMessage is actually delivered + the natural reply lands on a v0.5.11-rank-time alert.'
);

/* ---------------------------------------------------------------------- */
/* Prior-phase audit + memory-signal registry invariants                  */
/* ---------------------------------------------------------------------- */

const requiredCarryForwardActions = [
  // v0.5.3 hardening (still required)
  'fomo.sendblue.contact_registered',
  'fomo.sendblue.contact_registration_failed',
  'fomo.send.contact_not_registered',
  'fomo.oauth.refreshed',
  'fomo.oauth.refresh_failed',
  'fomo.db.connection_error',
  'fomo.sendblue.delivery_gap_detected',
  // v0.5.5 STOP enforcement
  'fomo.sendblue.stop_confirmation_sent',
  'fomo.sendblue.stop_confirmation_failed',
  'fomo.alert.suppressed_stop_active',
  'fomo.poll.skipped_stop_active',
  // v0.5.6 schema-violation fallback
  'fomo.alert.drafter_schema_failed',
  // v0.5.7 HMR degradation
  'fomo.alert.hmr_degradation_applied',
  // v0.5.8 Gmail INBOX event reliability
  'fomo.gmail.poll.event_observed',
  'fomo.gmail.poll.event_skipped',
  // v0.5.9 feedback substrate (still required)
  'feedback.written',
  'brevio.feedback.applied'
] as const satisfies readonly AuditAction[];
const auditActionSet = new Set<string>(FOMO_AUDIT_ACTIONS as readonly string[]);
const missingCarryForward = requiredCarryForwardActions.filter((a) => !auditActionSet.has(a));
if (missingCarryForward.length > 0) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'error',
    message: `Prior-phase audit actions missing from registry (still required for v0.5.11): ${missingCarryForward.join(', ')}`
  });
}

const memorySignalSet = new Set(MEMORY_SIGNAL_KINDS as readonly string[]);
const requiredCarryForwardMemoryKinds = [
  'stop_active',
  'sendblue_contact_status',
  // v0.5.9 invariant — Q2.B locks v0.5.9 sender_feedback_ignored UNTOUCHED
  // (no migration this phase). ignore_sender continues to upsert this kind
  // AND ALSO upserts the new sender_suppressed kind in v0.5.11.
  'sender_feedback_ignored'
] as const;
for (const k of requiredCarryForwardMemoryKinds) {
  if (!memorySignalSet.has(k)) {
    issues.push({
      name: 'MEMORY_SIGNAL_KINDS',
      severity: 'error',
      message: `Prior-phase memory_signal kind '${k}' missing from registry (still required for v0.5.11).`
    });
  }
}

// v0.5.7 HMR carry-forward.
const V057_TEMPLATE_VERSION_BASELINE = 'human-message-v0.3.0';
if (FOUNDER_TEXT_TEMPLATE_VERSION !== V057_TEMPLATE_VERSION_BASELINE) {
  issues.push({
    name: 'FOUNDER_TEXT_TEMPLATE_VERSION',
    severity: 'error',
    message:
      `v0.5.7 HMR carry-forward invariant: FOUNDER_TEXT_TEMPLATE_VERSION must remain '${V057_TEMPLATE_VERSION_BASELINE}'. Current: '${FOUNDER_TEXT_TEMPLATE_VERSION}'. v0.5.11 is PIL substrate + shadow eval; it MUST NOT touch the HMR template.`
  });
}

// v0.5.9 BREVIO_FEEDBACK_SURFACES + ACTIVE invariants.
if (BREVIO_FEEDBACK_SURFACES.length !== 13) {
  issues.push({
    name: 'BREVIO_FEEDBACK_SURFACES',
    severity: 'error',
    message: `v0.5.9 invariant: BREVIO_FEEDBACK_SURFACES must declare exactly 13 surfaces. Current count: ${BREVIO_FEEDBACK_SURFACES.length}.`
  });
}
if (BREVIO_FEEDBACK_ACTIVE_SURFACES.length !== 1 || BREVIO_FEEDBACK_ACTIVE_SURFACES[0] !== 'email_alert') {
  issues.push({
    name: 'BREVIO_FEEDBACK_ACTIVE_SURFACES',
    severity: 'error',
    message: `v0.5.9 invariant + v0.5.11 hard boundary: BREVIO_FEEDBACK_ACTIVE_SURFACES must be exactly ['email_alert']. Current: [${BREVIO_FEEDBACK_ACTIVE_SURFACES.join(',')}].`
  });
}

const v059GenericVerbs = ['approved', 'rejected', 'snoozed', 'ignored', 'asked_why', 'corrected'];
const kindsSet = new Set(BREVIO_FEEDBACK_EVENT_KINDS as readonly string[]);
const missingVerbs = v059GenericVerbs.filter((v) => !kindsSet.has(v));
if (missingVerbs.length > 0) {
  issues.push({
    name: 'BREVIO_FEEDBACK_EVENT_KINDS',
    severity: 'error',
    message: `v0.5.9 invariant: BREVIO_FEEDBACK_EVENT_KINDS must include the 6 generic verbs. Missing: ${missingVerbs.join(', ')}.`
  });
}

// v0.5.10 invariant: reply-parser-v0.2.0 is the required baseline for v0.5.11.
// The aggregation pipe consumes the positive-signal intents added in v0.5.10
// (this_mattered, more_like_this). If PROMPT_VERSION is still v0.1.0 the
// classifier won't produce those intents.
const V0510_PROMPT_VERSION_BASELINE = 'reply-parser-v0.2.0' as string;
const currentReplyParserVersion = REPLY_PARSER_PROMPT_VERSION as string;
if (currentReplyParserVersion !== V0510_PROMPT_VERSION_BASELINE) {
  issues.push({
    name: 'REPLY_PARSER_PROMPT_VERSION',
    severity: 'error',
    message:
      `v0.5.10 invariant: reply-parser PROMPT_VERSION must remain '${V0510_PROMPT_VERSION_BASELINE}'. Current: '${currentReplyParserVersion}'. v0.5.11 depends on the positive-signal intents (this_mattered, more_like_this) introduced in v0.5.10.`
  });
}

// v0.5.10 invariant: feedback-routing.ts policy module must be present;
// applyPilAggregation consumes feedback_events written by the routing module.
let v0510RoutingModulePresent = false;
try {
  const modulePath = '../src/reply-parser/feedback-routing.js';
  const mod = (await import(modulePath)) as Record<string, unknown>;
  v0510RoutingModulePresent = typeof mod.routeReplyFeedback === 'function';
} catch {
  v0510RoutingModulePresent = false;
}
if (!v0510RoutingModulePresent) {
  issues.push({
    name: 'V0510_FEEDBACK_ROUTING_MODULE',
    severity: 'error',
    message:
      'apps/fomo/src/reply-parser/feedback-routing.ts (v0.5.10 routeReplyFeedback) MUST be present — v0.5.11 aggregation pipe consumes feedback_events written by this module. If this is missing, v0.5.10 did not merge cleanly.'
  });
}

/* ---------------------------------------------------------------------- */
/* v0.5.11 — new tunable env vars (Q5.C)                                  */
/* ---------------------------------------------------------------------- */

checkIntBounds(
  'FOMO_PIL_K_THRESHOLD',
  3,
  1,
  null,
  "Minimum n_negative_events within recency window before false_positive aggregation flips sender_suppressed=true. Founder rule: don't let one correction flip a user's future behavior."
);
checkFloatBounds(
  'FOMO_PIL_SCORE_DELTA',
  0.1,
  0,
  0.5,
  'Per-event score shift on sender_importance.score. Conservative default 0.1; max 0.5. more_like_this applies 2× this delta.'
);
checkIntBounds(
  'FOMO_PIL_RECENCY_FULL_DAYS',
  90,
  1,
  null,
  'Days of full-weight contribution for the linear-decay model (Q3.B). Default 90 matches [[personalized-importance-learning]] §14.2 suggestion.'
);
checkIntBounds(
  'FOMO_PIL_RECENCY_DECAY_DAYS',
  90,
  0,
  null,
  'Additional days of linear decay after the full window. 0 = hard cliff. Default 90 → zero by day 180.'
);

/* ---------------------------------------------------------------------- */
/* v0.5.11-specific operator gate                                         */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_V0_5_11_BASELINE_CONFIRMED',
  'true',
  'v0.5.11 baseline gate: founder must run the runbook §1 baseline snapshot BEFORE the smoke starts. ' +
    'Snapshot records: existing feedback_events count + sender_feedback_ignored memory_signal count ' +
    "(v0.5.9 untouched invariant) + sender_importance row count (expected 0) + sender_suppressed row " +
    'count (expected 0) + brevio.signal.aggregated audit row count (expected 0) + ' +
    'alerts.sender_email_hash null-count (post-migration, pre-smoke). ' +
    'AFTER capture, set FOMO_V0_5_11_BASELINE_CONFIRMED=true and re-run preflight.'
);

/* ---------------------------------------------------------------------- */
/* PENDING runtime commit warnings                                        */
/* ---------------------------------------------------------------------- */

// brevio.signal.aggregated audit kind — runtime commit registers it.
const SIGNAL_AGGREGATED_KIND = 'brevio.signal.aggregated';
if (!auditActionSet.has(SIGNAL_AGGREGATED_KIND)) {
  issues.push({
    name: 'BREVIO_SIGNAL_AGGREGATED_AUDIT_KIND',
    severity: 'warn',
    message:
      `'${SIGNAL_AGGREGATED_KIND}' audit kind PENDING runtime commit registration in FOMO_AUDIT_ACTIONS. ` +
      `Q6.B locks 15 detail fields: verb, dimension, feedback_event_id, source_surface, memory_signal_kind, memory_signal_action, memory_signal_scope_key_hash, score_before, score_after, score_delta, n_positive_events_before/after, n_negative_events_before/after, suppression_flipped, threshold_k_in_force. ` +
      `After runtime + smoke PASS this warn disappears.`
  });
}

// Memory_signal kinds — SCAFFOLDING CORRECTION 2026-06-07:
// `sender_importance` and `sender_suppressed` are ALREADY registered in
// MEMORY_SIGNAL_KINDS as of v0.5.10. They are NOT new in v0.5.11. The v0.5.10
// `applyIgnoreSender` writes `sender_suppressed` with scope_key=`message:<id>`
// as an explicit v0.1 placeholder (sendblue-inbound.ts:1128). v0.5.11 ADDS
// the producer (aggregation pipe) AND TIGHTENS the scope_key contract from
// `message:<id>` to HMAC(sender_email). Existing placeholder rows stay as
// carry-forward; future hardening phase cleans them up.
// Preflight INVARIANT (not PENDING): both kinds must remain registered.
const PIL_KINDS_INVARIANT = ['sender_importance', 'sender_suppressed'] as const;
const missingPilKinds = PIL_KINDS_INVARIANT.filter((k) => !memorySignalSet.has(k));
if (missingPilKinds.length > 0) {
  issues.push({
    name: 'PIL_MEMORY_SIGNAL_KINDS_INVARIANT',
    severity: 'error',
    message:
      `PIL kinds MUST be in MEMORY_SIGNAL_KINDS (pre-existing as of v0.5.10): ${missingPilKinds.join(', ')}. ` +
      `If missing, a prior phase regressed the registry.`
  });
}

// pil-aggregation.ts module — runtime commit creates it.
let pilAggregationModulePresent = false;
try {
  const modulePath = '../src/memory/pil-aggregation.js';
  const mod = (await import(modulePath)) as Record<string, unknown>;
  pilAggregationModulePresent = typeof mod.applyPilAggregation === 'function';
} catch {
  pilAggregationModulePresent = false;
}
if (!pilAggregationModulePresent) {
  issues.push({
    name: 'PIL_AGGREGATION_MODULE',
    severity: 'warn',
    message:
      'apps/fomo/src/memory/pil-aggregation.ts PENDING runtime commit. ' +
      'Will export `applyPilAggregation(event, deps): Promise<PilAggregationOutcome>` — the aggregation consumer that turns v0.5.10 positive/correction feedback_events into sender_importance / sender_suppressed upserts. ' +
      'After runtime + smoke PASS this warn disappears.'
  });
}

// pil-context.ts shadow projection module — runtime commit creates it.
let pilContextModulePresent = false;
try {
  const modulePath = '../src/ranker/pil-context.js';
  const mod = (await import(modulePath)) as Record<string, unknown>;
  pilContextModulePresent = typeof mod.buildPilContext === 'function';
} catch {
  pilContextModulePresent = false;
}
if (!pilContextModulePresent) {
  issues.push({
    name: 'PIL_CONTEXT_MODULE',
    severity: 'warn',
    message:
      'apps/fomo/src/ranker/pil-context.ts PENDING runtime commit. ' +
      'Will export `buildPilContext(userId, senderEmailHash, deps): Promise<PilContext | null>` — pure projection reading sender_importance + sender_suppressed and applying Q3.B linear decay at read time. ' +
      'NEVER called by production ranker call site in v0.5.11 (live ranker passes pil_context: null). ' +
      'After runtime + smoke PASS this warn disappears.'
  });
}

// Shadow eval harness — runtime commit creates it.
issues.push({
  name: 'PIL_SHADOW_EVAL_HARNESS',
  severity: 'warn',
  message:
    'apps/fomo/src/eval/pil-shadow.eval.ts PENDING runtime commit. ' +
    'Will pair ≥10 synthetic email fixtures with synthetic PIL states and assert ranker shift DIRECTION (not exact magnitude — model nondeterminism allowed). ' +
    'Includes the LOAD-BEARING cross-user contamination row (per [[personalized-importance-learning]] §9.3, §10). ' +
    'Invoked via `pnpm --filter @brevio/fomo run eval:pil-shadow`. ' +
    'After runtime + smoke PASS this warn disappears.'
});

// alerts.sender_email_hash migration — runtime commit creates 0008.
issues.push({
  name: 'ALERTS_SENDER_EMAIL_HASH_MIGRATION',
  severity: 'warn',
  message:
    'apps/fomo/src/db/migrations/0008_alerts_sender_email_hash.sql PENDING runtime commit. ' +
    'Adds `sender_email_hash TEXT NULL` column to alerts (HMAC(sender_email, BREVIO_SENDER_HASH_KEY)). ' +
    'Backfill NULL on existing rows; populated forward by rank-step write. ' +
    'OPERATOR REMINDER: apply migration 0008 to live Neon BEFORE booting the dev server for the smoke (runbook §3). ' +
    'After runtime + migration applied + smoke PASS this warn disappears.'
});

// Live ranker bit-identical invariant — runtime commit must NOT mutate the
// production ranker call shape. Documented for operator visibility.
issues.push({
  name: 'LIVE_RANKER_BIT_IDENTICAL_INVARIANT',
  severity: 'warn',
  message:
    'LOCKED INVARIANT (hard boundary #1 + #2 + #3): production ranker call site MUST pass pil_context: null UNCONDITIONALLY in v0.5.11. ' +
    'PROMPT_VERSION stays ranker-v0.2.0 (no live bump). ' +
    'rank_results schema unchanged. ' +
    'sender_suppressed=true rows are NEVER read by live alert dispatch in v0.5.11. ' +
    'Verification: smoke-evidence C13 + C14 inspect call-site + dispatch path; eval harness is the ONLY consumer of buildPilContext + non-null pil_context. ' +
    'After runtime + smoke PASS this reminder disappears.'
});

/* ---------------------------------------------------------------------- */
/* Kill-switch sanity (carry-forward of v0.5.x scope-boundary checks)     */
/* ---------------------------------------------------------------------- */

if ((process.env.FOMO_AUTO_SEND_ENABLED ?? '').trim() === 'true') {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message:
      'FOMO_AUTO_SEND_ENABLED=true is forbidden in v0.5.11. Auto-send is its own future gate per FOMO_PLAN v0.8.'
  });
}

if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'error',
    message:
      'BREVIO_DEV_MODE=true skips Postgres persistence (in-memory stores). v0.5.11 smoke MUST run against real Neon Postgres so the new alerts.sender_email_hash column + sender_importance / sender_suppressed memory_signal writes + brevio.signal.aggregated audit rows are observable in the live DB.'
  });
}

/* ---------------------------------------------------------------------- */
/* Output                                                                  */
/* ---------------------------------------------------------------------- */

const errors = issues.filter((i) => i.severity === 'error');
const warns = issues.filter((i) => i.severity === 'warn');

if (errors.length > 0) {
  console.log('Preflight FAILED:\n');
  for (const i of errors) {
    console.log(`  [ERROR] ${i.name}: ${i.message}`);
  }
  if (warns.length > 0) {
    console.log('\nWarnings (non-blocking):');
    for (const i of warns) {
      console.log(`  [WARN]  ${i.name}: ${i.message}`);
    }
  }
  process.exit(1);
}

console.log('Resolved kill switches:');
const switches = loadKillSwitches(process.env);
console.log(JSON.stringify(switches, null, 2));
console.log('');

if (warns.length > 0) {
  console.log('Warnings (PENDING runtime commit / operator reminders):');
  for (const i of warns) {
    console.log(`  [WARN] ${i.name}: ${i.message}`);
  }
  console.log('');
}

console.log('✓ Preflight passed.\n');
console.log('  Next steps (see docs/smoke-test-v0.5.11-pil-substrate-shadow.md):');
console.log('    1. Migration 0008 (alerts.sender_email_hash) applied to live Neon — verify via §3');
console.log('    2. §1 baseline snapshot captured (FOMO_V0_5_11_BASELINE_CONFIRMED=true)');
console.log('    3. Boot dev server: pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.11.log');
console.log('    4. Boot ngrok (or use existing tunnel) so SendBlue inbound can reach the dev server');
console.log('    5. Path A (LOAD-BEARING ignore_sender natural reply on v0.5.11-rank-time alert) — runbook §6');
console.log('    6. Path B (positive intents via ops:feedback-inject + natural reply where possible) — runbook §7');
console.log('    7. Path C (threshold-k test: 3 consecutive false_positive on a synthetic sender flips sender_suppressed) — runbook §8');
console.log('    8. Path D (shadow eval harness) — `pnpm --filter @brevio/fomo run eval:pil-shadow` — runbook §9');
console.log('    9. Path E (cross-user contamination) — runbook §10');
console.log('   10. Carry-forward: pnpm smoke-evidence:v0.5.9 + smoke-evidence:v0.5.10');
console.log('   11. pnpm smoke-evidence:v0.5.11 — expect VERDICT: PASS');
console.log('   12. Fill in docs/SMOKE_REPORT_v0.5.11.md');
