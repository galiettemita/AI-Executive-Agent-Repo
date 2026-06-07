// Phase v0.5.12 preflight — Live ranker reads PIL in guarded mode (founder-only smoke).
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-07):
//   This script is part of the v0.5.12 SCAFFOLDING commit, which lands BEFORE
//   the runtime implementation. v0.5.12 is the FIRST phase where the live
//   ranker actually reads PIL signals at rank time. v0.5.11 made PIL substrate
//   + shadow eval live; v0.5.12 turns it into bounded, eval-gated, kill-
//   switchable production behavior.
//
//   Locked scope (see memory project_v05-12-scope):
//     Q1.C-modified: hybrid with REAL clamp.
//       When FOMO_PIL_LIVE_ENABLED=true AND non-null PIL context exists, the
//       ranker path MUST run BOTH a baseline (pil_context: null) AND a PIL-
//       context call, then clamp(pil_score - baseline_score) at
//       ±FOMO_PIL_SCORE_CAP. The two-call shape is the only way the cap is
//       real — prompt-only instructions are not sufficient. Doubles OpenAI
//       cost ONLY in the PIL-influenced rank path; baseline-only ranks remain
//       single-call.
//     Q2.A: FOMO_PIL_SCORE_CAP default 0.15, bounds [0.05, 0.25].
//     Q3.A + Q3.C: sender_suppressed=true is a STRONG PRIOR not a dispatch-
//       time block. Model can override on strong intrinsic signal. Recency
//       decay (already implemented in v0.5.11 buildPilContext) ensures stale
//       signals contribute zero. EXPLICITLY REJECTED: dispatch-time
//       filtering / binary blindness on suppression.
//     Q4.A: offline fixture diff eval (pil-live.eval.ts extends shadow
//       harness with live model calls). Production divergence audit may be
//       added but DISABLED BY DEFAULT (FOMO_PIL_DIVERGENCE_AUDIT_ENABLED=false).
//     Q5.A: FOMO_PIL_LIVE_ENABLED=false default global kill switch.
//       When false: pil_context: null, prompt = v0.5.11 baseline, no audit,
//       behavior bit-identical.
//     Q6.A: new audit kind brevio.rank.pil_applied with 9 locked structural
//       fields. NO rank_results schema change (preserves v0.5.11 invariant).
//
//   Critical implementation rule (read-side filter):
//     buildLivePilContext(userId, senderEmailHash, deps) MUST filter to
//     memory_signals.scope_key ~ '^[a-f0-9]{32}$' AND user_id = userId.
//     Legacy scope_key='message:<id>' placeholder rows from v0.5.10
//     applyIgnoreSender MUST be ignored.
//
//   The runtime commit(s) will:
//     - Add apps/fomo/src/ranker/pil-live-context.ts (or extend
//       pil-context.ts) exposing buildLivePilContext with the canonical-HMAC
//       read-side filter AND the FOMO_PIL_LIVE_ENABLED unconditional null-
//       return when the kill switch is off.
//     - Bump PROMPT_VERSION from 'ranker-v0.2.0' to 'ranker-v0.3.0' ONLY
//       when pil_context is non-null at prompt assembly time. Baseline (no
//       pil_context) calls continue to assemble as ranker-v0.2.0 shape.
//     - Wire the two-call hybrid at the production ranker call site: when
//       PIL applies, run baseline + PIL calls and clamp.
//     - Register brevio.rank.pil_applied in FOMO_AUDIT_ACTIONS with 9
//       locked detail fields.
//     - Add apps/fomo/src/eval/pil-live.eval.ts — extends pil-shadow.eval.ts
//       with live ranker calls + BB1–BB8 becomes-blind fixtures.
//     - NEW env knobs: FOMO_PIL_LIVE_ENABLED, FOMO_PIL_SCORE_CAP.
//
//   While the runtime commit is pending:
//     - pil-live-context module not present → WARN PENDING (dynamic probe)
//     - pil-live.eval.ts not present → WARN PENDING
//     - brevio.rank.pil_applied not in FOMO_AUDIT_ACTIONS → WARN PENDING
//     - PROMPT_VERSION still 'ranker-v0.2.0' → PASS during scaffolding;
//       AFTER runtime ships, the version bump is conditional on the prompt
//       carrying a pil_context block; the production prompt assembly module
//       must export both shapes
//
//   When runtime + smoke + report land, all WARNs flip to silent.
//
// Pure config inspection — no DB, no network. Validates that:
//   (a) the substrate is the v0.5.11-PASS shape (so v0.5.12 layers on a
//       known-good base)
//   (b) the v0.5.12-specific founder gate is set
//       (FOMO_V0_5_12_BASELINE_CONFIRMED)
//   (c) the two new tunable env vars parse + are within bounds
//   (d) carry-forward invariants (HMR template, BREVIO_FEEDBACK surfaces,
//       reply-parser-v0.2.0, PIL substrate kinds, brevio.signal.aggregated
//       audit registration) still hold
//
// Forbidden in v0.5.12 (preserves prior phase locks + permanent product
// principles):
//   * HMR / renderer / drafter changes (3E.1 invariant)
//   * User-facing feedback acknowledgment text (Q5.A v0.5.10 defer)
//   * Auto-send (own future gate)
//   * New tools / new modalities
//   * New active source_surface beyond email_alert
//   * Reply-parser changes (PROMPT_VERSION='reply-parser-v0.2.0' invariant)
//   * New memory_signal kinds (v0.5.12 is READ-ONLY against v0.5.11 substrate)
//   * Reading legacy scope_key='message:<id>' placeholder rows
//   * Raw private content (sender_email, subject, body, snippet, headers) in
//     ranker prompt or audit detail
//   * Global learning / cross-user signal pooling
//   * Dispatch-time hard block from sender_suppressed (strong prior only)
//   * One-event suppression (k-threshold + decay enforced at substrate)
//   * FOMO_PIL_DIVERGENCE_AUDIT_ENABLED=true by default
//   * Backfill of pre-migration alerts.sender_email_hash NULL rows
//   * FOMO_AUTO_SEND_ENABLED=true
//   * BREVIO_DEV_MODE=true (smoke runs against real Neon)
//   * Friend C onboarding (three-friend cap)
//
// The runbook (docs/smoke-test-v0.5.12-live-ranker-pil-guarded.md) covers
// out-of-band requirements preflight cannot check:
//   * Baseline snapshot of rank_results + brevio.signal.aggregated +
//     brevio.rank.pil_applied (expected 0 before smoke) captured (so post-
//     smoke diff proves new rows are from the smoke)
//   * v0.5.11 substrate carry-forward: at least one canonical-HMAC PIL row
//     present from prior v0.5.11 smoke (or seeded synthetically) so the live
//     ranker has something to read
//   * SendBlue + Slack carry-forward env unchanged from v0.5.11
//   * No friend involved this phase (three-friend cap; founder-only smoke)

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
import {
  PROMPT_VERSION as RANKER_PROMPT_VERSION,
  PROMPT_VERSION_WITH_PIL as RANKER_PROMPT_VERSION_WITH_PIL
} from '../src/ranker/prompt.js';

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
      message: `${name} required for v0.5.12 (${ctx}). Set ≥ ${min} explicitly.`
    });
    return;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || n < min) {
    issues.push({
      name,
      severity: 'error',
      message: `${name}=${raw} is below the v0.5.12 minimum of ${min}. ${ctx}`
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
  if (!Number.isFinite(n) || n < min || n > max) {
    issues.push({
      name,
      severity: 'error',
      message: `${name}='${raw}' must be a finite number in [${min}, ${max}]. ${ctx}`
    });
  }
}
function expectBoolFalseOrTrue(
  name: string,
  ctx: string
): void {
  const raw = (process.env[name] ?? '').trim();
  if (raw === '') return; // unset → default false (PASS — kill switch off is the safe baseline)
  if (raw !== 'true' && raw !== 'false') {
    issues.push({
      name,
      severity: 'error',
      message: `${name}='${raw}' must be 'true' or 'false' (string). ${ctx}`
    });
  }
}

console.log('Phase v0.5.12 preflight — Live ranker reads PIL in guarded mode (founder-only smoke)\n');

/* ---------------------------------------------------------------------- */
/* Substrate carry-forward (v0.5.1 through v0.5.11)                       */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required.');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');
requireMin('BREVIO_PHONE_HASH_KEY', 32, 'BREVIO_PHONE_HASH_KEY must be a 32-byte key');
// LOAD-BEARING for v0.5.12: the live ranker's PIL context lookup HMACs the
// sender_email at read time. The hash MUST match the value the v0.5.11 rank-
// step wrote to alerts.sender_email_hash AND the v0.5.11 aggregation pipe
// wrote to memory_signals.scope_key. Drift here = the live ranker reads zero
// PIL signals despite the substrate being populated → false-negative blindness.
requireMin(
  'BREVIO_SENDER_HASH_KEY',
  32,
  'BREVIO_SENDER_HASH_KEY must be a 32-byte key. v0.5.12 LOAD-BEARING — used to hash sender_email at live-ranker read time. Hash MUST match v0.5.11 rank-write hash AND aggregation-pipe scope_key hash. Drift here = silent zero-PIL-context blindness.'
);

expectEquals(
  'FOMO_FRIEND_BETA_ENABLED',
  'true',
  'v0.5.12 substrate still requires the friend-beta kill switch ON (carry-forward; no friend involved this phase but substrate stays live).'
);

require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required.');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required.');
require_('BREVIO_OAUTH_REDIRECT_URI_GOOGLE', 'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required.');

require_('SENDBLUE_API_KEY_ID', 'SENDBLUE_API_KEY_ID required.');
require_('SENDBLUE_API_SECRET_KEY', 'SENDBLUE_API_SECRET_KEY required.');
require_('SENDBLUE_FROM_NUMBER', 'SENDBLUE_FROM_NUMBER required.');
require_('SENDBLUE_WEBHOOK_SECRET', 'SENDBLUE_WEBHOOK_SECRET required.');

require_(
  'OPENAI_API_KEY',
  'OPENAI_API_KEY required. v0.5.12 LOAD-BEARING — the two-call hybrid runs TWO ranker calls per PIL-influenced rank (baseline + PIL). The offline pil-live.eval.ts also calls OpenAI.'
);
expectEquals('FOMO_RANKER_ENABLED', 'true', 'Ranker must be on — this phase is the ranker integration.');
expectEquals(
  'FOMO_SLACK_REVIEW_ENABLED',
  'true',
  'Slack review must be on — substrate live for the smoke alert produced by the ranker.'
);
expectEquals(
  'FOMO_SEND_ENABLED',
  'true',
  'FOMO_SEND_ENABLED must be true so the outbound worker can produce the alert messages the smoke observes.'
);
expectEquals(
  'FOMO_GMAIL_POLLING_ENABLED',
  'true',
  'Polling must be on so the alerts under v0.5.12 smoke get surfaced.'
);
expectEquals(
  'FOMO_SENDBLUE_INBOUND_ENABLED',
  'true',
  'FOMO_SENDBLUE_INBOUND_ENABLED must be true — carry-forward from v0.5.11.'
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
      'FOMO_FOUNDER_PHONE_NUMBER required (carry-forward).'
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
      'FOMO_FRIEND_BETA_BASE_URL required (substrate live carry-forward).'
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
  'v0.5.12 smoke needs the polling worker live long enough for at least one alert to be ranked + delivered so the live two-call hybrid is exercised on a fresh row.'
);
checkCycleMin(
  'FOMO_OUTBOUND_MAX_CYCLES',
  30,
  'v0.5.12 smoke keeps the outbound worker live to deliver the alert messages.'
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
  'brevio.feedback.applied',
  // v0.5.11 PIL substrate (still required — write path stays live)
  'brevio.signal.aggregated'
] as const satisfies readonly AuditAction[];
const auditActionSet = new Set<string>(FOMO_AUDIT_ACTIONS as readonly string[]);
const missingCarryForward = requiredCarryForwardActions.filter((a) => !auditActionSet.has(a));
if (missingCarryForward.length > 0) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'error',
    message: `Prior-phase audit actions missing from registry (still required for v0.5.12): ${missingCarryForward.join(', ')}`
  });
}

const memorySignalSet = new Set(MEMORY_SIGNAL_KINDS as readonly string[]);
const requiredCarryForwardMemoryKinds = [
  'stop_active',
  'sendblue_contact_status',
  'sender_feedback_ignored',
  // v0.5.11 substrate kinds — load-bearing READ source for v0.5.12 live ranker
  'sender_importance',
  'sender_suppressed'
] as const;
for (const k of requiredCarryForwardMemoryKinds) {
  if (!memorySignalSet.has(k)) {
    issues.push({
      name: 'MEMORY_SIGNAL_KINDS',
      severity: 'error',
      message: `Prior-phase memory_signal kind '${k}' missing from registry (still required for v0.5.12 — v0.5.11 substrate is the read source).`
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
      `v0.5.7 HMR carry-forward invariant: FOUNDER_TEXT_TEMPLATE_VERSION must remain '${V057_TEMPLATE_VERSION_BASELINE}'. Current: '${FOUNDER_TEXT_TEMPLATE_VERSION}'. v0.5.12 is live-ranker-reads-PIL; it MUST NOT touch HMR.`
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
    message: `v0.5.9 invariant + v0.5.12 hard boundary: BREVIO_FEEDBACK_ACTIVE_SURFACES must be exactly ['email_alert']. Current: [${BREVIO_FEEDBACK_ACTIVE_SURFACES.join(',')}].`
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

// v0.5.10 invariant: reply-parser-v0.2.0 unchanged.
const V0510_PROMPT_VERSION_BASELINE = 'reply-parser-v0.2.0' as string;
const currentReplyParserVersion = REPLY_PARSER_PROMPT_VERSION as string;
if (currentReplyParserVersion !== V0510_PROMPT_VERSION_BASELINE) {
  issues.push({
    name: 'REPLY_PARSER_PROMPT_VERSION',
    severity: 'error',
    message:
      `v0.5.10 invariant: reply-parser PROMPT_VERSION must remain '${V0510_PROMPT_VERSION_BASELINE}'. Current: '${currentReplyParserVersion}'. v0.5.12 does NOT touch the reply parser.`
  });
}

// v0.5.10 routing module present.
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
      'apps/fomo/src/reply-parser/feedback-routing.ts (v0.5.10 routeReplyFeedback) MUST be present.'
  });
}

// v0.5.11 substrate modules present — v0.5.12 READS from them.
let v0511AggregationModulePresent = false;
try {
  const modulePath = '../src/memory/pil-aggregation.js';
  const mod = (await import(modulePath)) as Record<string, unknown>;
  v0511AggregationModulePresent = typeof mod.applyPilAggregation === 'function';
} catch {
  v0511AggregationModulePresent = false;
}
if (!v0511AggregationModulePresent) {
  issues.push({
    name: 'V0511_PIL_AGGREGATION_MODULE',
    severity: 'error',
    message:
      'apps/fomo/src/memory/pil-aggregation.ts (v0.5.11 applyPilAggregation) MUST be present. v0.5.12 layers on top of the v0.5.11 substrate; absent module = v0.5.11 did not merge cleanly.'
  });
}

let v0511ShadowContextModulePresent = false;
try {
  const modulePath = '../src/ranker/pil-context.js';
  const mod = (await import(modulePath)) as Record<string, unknown>;
  v0511ShadowContextModulePresent = typeof mod.buildPilContext === 'function';
} catch {
  v0511ShadowContextModulePresent = false;
}
if (!v0511ShadowContextModulePresent) {
  issues.push({
    name: 'V0511_PIL_CONTEXT_MODULE',
    severity: 'error',
    message:
      'apps/fomo/src/ranker/pil-context.ts (v0.5.11 buildPilContext) MUST be present. v0.5.12 reuses the projection + decay logic from this module.'
  });
}

// v0.5.11 audit kind brevio.signal.aggregated still registered.
if (!auditActionSet.has('brevio.signal.aggregated')) {
  issues.push({
    name: 'V0511_BREVIO_SIGNAL_AGGREGATED',
    severity: 'error',
    message:
      "brevio.signal.aggregated must remain in FOMO_AUDIT_ACTIONS (v0.5.11 substrate write path stays live in v0.5.12)."
  });
}

/* ---------------------------------------------------------------------- */
/* v0.5.12 — new tunable env vars (Q2.A + Q5.A)                           */
/* ---------------------------------------------------------------------- */

// FOMO_PIL_SCORE_CAP — Q2.A hard cap, bounds [0.05, 0.25], default 0.15.
checkFloatBounds(
  'FOMO_PIL_SCORE_CAP',
  0.15,
  0.05,
  0.25,
  'Q2.A hard cap on absolute PIL score delta after clamp. Default 0.15. Founder lock: cap is enforced at the rank-write step after model output; prompt-only instructions are not sufficient.'
);

// FOMO_PIL_LIVE_ENABLED — Q5.A global kill switch, default false.
expectBoolFalseOrTrue(
  'FOMO_PIL_LIVE_ENABLED',
  'Q5.A global kill switch. Default false (omitted = false). When false: pil_context: null unconditionally, prompt assembly = v0.5.11 baseline shape (single ranker call, no PIL block), no brevio.rank.pil_applied audit fires, behavior bit-identical to v0.5.11. When true: two-call hybrid runs on PIL-influenced ranks.'
);

// FOMO_PIL_DIVERGENCE_AUDIT_ENABLED — Q4.A documented but off-by-default.
expectBoolFalseOrTrue(
  'FOMO_PIL_DIVERGENCE_AUDIT_ENABLED',
  'Q4.A documented but disabled by default. When true: every rank emits a divergence audit. NOT load-bearing this phase. Founder lock: production divergence audit is NOT default-on.'
);
if ((process.env.FOMO_PIL_DIVERGENCE_AUDIT_ENABLED ?? '').trim() === 'true') {
  issues.push({
    name: 'FOMO_PIL_DIVERGENCE_AUDIT_ENABLED',
    severity: 'warn',
    message:
      'FOMO_PIL_DIVERGENCE_AUDIT_ENABLED=true is documented but NOT load-bearing this phase. Every rank emits a heavyweight divergence audit. Use only for short observation windows.'
  });
}

// Carry-forward v0.5.11 tunables (still load-bearing for the substrate write path).
checkFloatBounds(
  'FOMO_PIL_SCORE_DELTA',
  0.1,
  0,
  0.5,
  'v0.5.11 substrate carry-forward. Per-event score shift on sender_importance.score; unchanged this phase.'
);

/* ---------------------------------------------------------------------- */
/* v0.5.12-specific operator gate                                         */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_V0_5_12_BASELINE_CONFIRMED',
  'true',
  'v0.5.12 baseline gate: founder must run the runbook §1 baseline snapshot BEFORE the smoke starts. ' +
    'Snapshot records: existing rank_results count + brevio.signal.aggregated audit count + ' +
    'brevio.rank.pil_applied audit count (expected 0 before smoke) + sender_importance / sender_suppressed ' +
    'row count + count of alerts.sender_email_hash NOT NULL (the rows the live ranker COULD read PIL for). ' +
    'AFTER capture, set FOMO_V0_5_12_BASELINE_CONFIRMED=true and re-run preflight.'
);

/* ---------------------------------------------------------------------- */
/* PENDING runtime commit warnings                                        */
/* ---------------------------------------------------------------------- */

// brevio.rank.pil_applied audit kind — runtime commit registers it.
const PIL_APPLIED_KIND = 'brevio.rank.pil_applied';
if (!auditActionSet.has(PIL_APPLIED_KIND)) {
  issues.push({
    name: 'BREVIO_RANK_PIL_APPLIED_AUDIT_KIND',
    severity: 'warn',
    message:
      `'${PIL_APPLIED_KIND}' audit kind PENDING runtime commit registration in FOMO_AUDIT_ACTIONS. ` +
      `Q6.A locks 9 structural detail fields: rank_result_id, pil_signal_kinds_present, score_before_pil_cap, score_after_pil_cap, pil_score_delta, pil_score_delta_was_capped, model_mentioned_pil_in_reason, source_surface, scope_key_hash. ` +
      `Forbidden in detail: raw sender_email, subject, body, snippet, headers, raw rank.reason text. ` +
      `After runtime + smoke PASS this warn disappears.`
  });
}

// pil-live-context module — runtime commit creates it.
let pilLiveContextModulePresent = false;
try {
  const modulePath = '../src/ranker/pil-live-context.js';
  const mod = (await import(modulePath)) as Record<string, unknown>;
  pilLiveContextModulePresent = typeof mod.buildLivePilContext === 'function';
} catch {
  pilLiveContextModulePresent = false;
}
if (!pilLiveContextModulePresent) {
  // The runtime commit may instead extend pil-context.ts with buildLivePilContext;
  // probe that path too.
  try {
    const modulePath = '../src/ranker/pil-context.js';
    const mod = (await import(modulePath)) as Record<string, unknown>;
    pilLiveContextModulePresent = typeof mod.buildLivePilContext === 'function';
  } catch {
    pilLiveContextModulePresent = false;
  }
}
if (!pilLiveContextModulePresent) {
  issues.push({
    name: 'PIL_LIVE_CONTEXT_MODULE',
    severity: 'warn',
    message:
      'buildLivePilContext PENDING runtime commit. ' +
      'Will export `buildLivePilContext(userId, senderEmailHash, deps): Promise<PilContext | null>` — ' +
      'the read-side filter MUST enforce scope_key ~ ^[a-f0-9]{32}$ (canonical HMAC shape) AND ' +
      'return null unconditionally when FOMO_PIL_LIVE_ENABLED=false. ' +
      'Legacy scope_key="message:<id>" placeholder rows from v0.5.10 applyIgnoreSender MUST be ignored (BB6 LOAD-BEARING). ' +
      'Runtime ships either in pil-live-context.ts (new file) or as an extension of pil-context.ts. ' +
      'After runtime + smoke PASS this warn disappears.'
  });
}

// pil-live.eval.ts harness — runtime commit creates it.
let pilLiveEvalPresent = false;
try {
  const modulePath = '../src/eval/pil-live.eval.js';
  await import(modulePath);
  pilLiveEvalPresent = true;
} catch {
  pilLiveEvalPresent = false;
}
if (!pilLiveEvalPresent) {
  issues.push({
    name: 'PIL_LIVE_EVAL_HARNESS',
    severity: 'warn',
    message:
      'apps/fomo/src/eval/pil-live.eval.ts PENDING runtime commit. ' +
      'Will extend pil-shadow.eval.ts with live ranker calls + the 8 LOAD-BEARING becomes-blind fixtures: ' +
      'BB1 (suppressed sender + URGENT email → model overrides), ' +
      'BB2 (score=-0.3 not auto-dropped), ' +
      'BB3 (200d-old signal decays to ≈0), ' +
      'BB4 (cross-user read isolation), ' +
      'BB5 (1 false_positive within noise floor), ' +
      'BB6 (legacy message:<id> row → null PIL context), ' +
      'BB7 (kill switch off → bit-identical baseline), ' +
      'BB8 (max-score input clamped at cap). ' +
      'Invoked via `pnpm --filter @brevio/fomo run eval:pil-live`. ' +
      'After runtime + smoke PASS this warn disappears.'
  });
}

// Ranker prompt version invariant.
//   - PROMPT_VERSION (baseline / no PIL block)  MUST == 'ranker-v0.2.0'
//   - PROMPT_VERSION_WITH_PIL (PIL block included) MUST == 'ranker-v0.3.0'
// Both constants ship as of v0.5.12 runtime. If either drifts, fail loud.
const EXPECTED_V0511_RANKER_PROMPT_VERSION = 'ranker-v0.2.0' as string;
const EXPECTED_V0512_RANKER_PROMPT_VERSION = 'ranker-v0.3.0' as string;
const currentRankerPromptVersion = RANKER_PROMPT_VERSION as string;
const currentRankerPromptVersionWithPil = RANKER_PROMPT_VERSION_WITH_PIL as string;
if (currentRankerPromptVersion !== EXPECTED_V0511_RANKER_PROMPT_VERSION) {
  issues.push({
    name: 'RANKER_PROMPT_VERSION',
    severity: 'error',
    message:
      `Ranker baseline PROMPT_VERSION must be '${EXPECTED_V0511_RANKER_PROMPT_VERSION}' (the no-PIL-block shape). Current: '${currentRankerPromptVersion}'.`
  });
}
if (currentRankerPromptVersionWithPil !== EXPECTED_V0512_RANKER_PROMPT_VERSION) {
  issues.push({
    name: 'RANKER_PROMPT_VERSION_WITH_PIL',
    severity: 'error',
    message:
      `Ranker PIL-block PROMPT_VERSION_WITH_PIL must be '${EXPECTED_V0512_RANKER_PROMPT_VERSION}'. Current: '${currentRankerPromptVersionWithPil}'. The two-call hybrid emits baseline calls as 'ranker-v0.2.0' and PIL calls as 'ranker-v0.3.0' to keep cost_records attributable to the source prompt.`
  });
}

// Live ranker bit-identical invariant — runtime commit must respect kill switch.
issues.push({
  name: 'LIVE_RANKER_KILL_SWITCH_INVARIANT',
  severity: 'warn',
  message:
    'LOCKED INVARIANT (Q5.A + C1): when FOMO_PIL_LIVE_ENABLED=false (default), ' +
    'production ranker call site MUST pass pil_context: null UNCONDITIONALLY (single ranker call, no PIL block in prompt, no brevio.rank.pil_applied audit). ' +
    'Bit-identical to v0.5.11 behavior. ' +
    'Verification: smoke-evidence C1 inspects rank_results + audit rows produced while kill switch was off. ' +
    'After runtime + smoke PASS this reminder disappears.'
});

// Read-side filter invariant — runtime commit must enforce.
issues.push({
  name: 'PIL_LIVE_READ_SIDE_FILTER_INVARIANT',
  severity: 'warn',
  message:
    'LOCKED INVARIANT (founder rule): buildLivePilContext MUST filter memory_signals to scope_key ~ ^[a-f0-9]{32}$ (canonical HMAC shape). ' +
    'Legacy scope_key="message:<id>" placeholder rows MUST be ignored. ' +
    'Verification: smoke-evidence C3 + BB6 fixture in pil-live.eval.ts. ' +
    'After runtime + smoke PASS this reminder disappears.'
});

// Two-call hybrid invariant — runtime commit must enforce.
issues.push({
  name: 'TWO_CALL_HYBRID_INVARIANT',
  severity: 'warn',
  message:
    'LOCKED INVARIANT (Q1.C-modified founder rule): when FOMO_PIL_LIVE_ENABLED=true AND pil_context is non-null, ' +
    'the production ranker path MUST run BOTH a baseline (pil_context: null) AND a PIL-context call. ' +
    'final_delta = clamp(pil_score - baseline_score, -FOMO_PIL_SCORE_CAP, +FOMO_PIL_SCORE_CAP). ' +
    'final_score = baseline_score + final_delta. ' +
    'If the baseline call is skipped, the cap is not real → Q1.C is rejected. ' +
    'Verification: smoke-evidence C4 + BB8 fixture. ' +
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
      'FOMO_AUTO_SEND_ENABLED=true is forbidden in v0.5.12. Auto-send is its own future gate.'
  });
}

if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'error',
    message:
      'BREVIO_DEV_MODE=true skips Postgres persistence. v0.5.12 smoke MUST run against real Neon so the new brevio.rank.pil_applied audit rows are observable in the live DB.'
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
console.log('  Next steps (see docs/smoke-test-v0.5.12-live-ranker-pil-guarded.md):');
console.log('    1. §1 baseline snapshot captured (FOMO_V0_5_12_BASELINE_CONFIRMED=true)');
console.log('    2. Verify ≥1 canonical-HMAC PIL row exists in memory_signals (carry-forward from v0.5.11 smoke or synthetic seed). LOAD-BEARING — live ranker has nothing to read if absent.');
console.log('    3. Boot dev server: pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.12.log');
console.log('    4. Path A — Kill switch OFF (FOMO_PIL_LIVE_ENABLED=false): drive a rank, verify bit-identical v0.5.11 behavior (no PIL prompt block, no brevio.rank.pil_applied audit). C1 LOAD-BEARING. — runbook §5');
console.log('    5. Path B — Kill switch ON, NO matching PIL row: drive a rank, verify pil_context=null and no audit. — runbook §6');
console.log('    6. Path C — Kill switch ON, canonical-HMAC PIL row exists: drive a rank, verify two-call hybrid runs, score capped, audit fires with all 9 fields. C2 + C4 + C5 LOAD-BEARING. — runbook §7');
console.log('    7. Path D — Kill switch ON, legacy scope_key="message:<id>" row only: drive a rank, verify pil_context=null and no audit. C3 LOAD-BEARING. — runbook §8');
console.log('    8. Path E — Cross-user contamination: user B rank with user A signal in DB → null context. C7 LOAD-BEARING. — runbook §9');
console.log('    9. Path F — Offline eval: pnpm --filter @brevio/fomo run eval:pil-live → 11 carry-forward fixtures + BB1–BB8 LOAD-BEARING becomes-blind. — runbook §10');
console.log('   10. Carry-forward: pnpm smoke-evidence:v0.5.9 + smoke-evidence:v0.5.10 + smoke-evidence:v0.5.11');
console.log('   11. pnpm smoke-evidence:v0.5.12 — expect VERDICT: PASS');
console.log('   12. Fill in docs/SMOKE_REPORT_v0.5.12.md');
