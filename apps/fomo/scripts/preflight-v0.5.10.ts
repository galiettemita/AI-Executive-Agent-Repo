// Phase v0.5.10 preflight — Reply-parser feedback intents (founder-only smoke).
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-06):
//   This script is part of the v0.5.10 SCAFFOLDING commit, which lands BEFORE
//   the runtime implementation. v0.5.10 connects the existing v0.1.0 reply-
//   parser (Phase 3F.1; classifies 6 intents into snooze/ignore/ignore_sender/
//   why/false_positive/unclear) into the v0.5.9 Brevio-wide Feedback substrate
//   (`feedback_events.source_surface` + `BREVIO_FEEDBACK_EVENT_KINDS` +
//   `applyFeedback`). It ALSO adds 2 NEW positive-signal intents to the
//   classifier: `this_mattered` and `more_like_this`.
//
//   The runtime commit will:
//     - Bump PROMPT_VERSION 'reply-parser-v0.1.0' → 'reply-parser-v0.2.0'
//     - Extend BREVIO_FEEDBACK_EVENT_KINDS / dimension union to include
//       'importance' | 'pattern'
//     - Add `apps/fomo/src/reply-parser/feedback-routing.ts` policy module
//     - Add the Q3.C explicit-feedback-phrase allowlist to
//       `parseReplyDeterministic`
//     - Add the orchestrator ≤3-word safe rule
//     - Route sendblue-inbound.ts intent handling through the new module
//     - Add `intent_source` audit field to slack-interactivity +
//       ops-feedback-inject for symmetry
//   No new env var is required. No new audit kind (Q6.A-modified extends
//   the existing `feedback.written` detail with 10 locked fields).
//
//   While the runtime commit is pending:
//     - PROMPT_VERSION === 'reply-parser-v0.1.0' (NOT v0.2.0) → WARN PENDING
//     - feedback-routing.ts not present → WARN PENDING (dynamic import probe)
//     - Allowlist not in deterministic.ts → WARN PENDING (grep marker probe)
//
//   When the runtime commit lands and the smoke runs end-to-end, all WARNs
//   flip to silent except the migration-style operator reminders.
//
// Pure config inspection — no DB, no network. Validates that:
//   (a) the substrate is the v0.5.9-PASS shape (so the v0.5.10 smoke runs
//       against a known-good substrate)
//   (b) the v0.5.10-specific founder gate is set (FOMO_V0_5_10_BASELINE_CONFIRMED)
//   (c) carry-forward audit + memory_signal invariants still hold
//
// v0.5.10 scope (locked Q1–Q6 — see memory project_v05-10-scope):
//   * Q1.B — new `feedback-routing.ts` policy module is the single testable
//     place for "intent → feedback_event_input + applyFeedback" decisions.
//     Parser stays stateless.
//   * Q2.A-modified — PROMPT_VERSION bumps to 'reply-parser-v0.2.0'; intent
//     set extends 6 → 8 (adds `this_mattered`, `more_like_this`). Founder
//     CORRECTION absorbed: `this_mattered` is POSITIVE confirmation, NOT a
//     negative correction. Mappings:
//       - this_mattered    → verb=approved, dimension=importance,
//                            role=user, value=confirmed_important
//       - more_like_this   → verb=approved, dimension=pattern,
//                            role=user, value=more_like_this
//       - false_positive   → verb=corrected, dimension=ranker_label,
//                            previous_label=important, corrected_label=not_important,
//                            role=user
//       - ignore_sender    → verb=ignored, dimension=sender, role=user
//                            → TRIGGERS v0.5.9 applyFeedback
//                            → sender_feedback_ignored memory_signal upsert
//       - ignore           → verb=ignored, dimension=alert, role=user
//       - snooze           → verb=snoozed, dimension=alert, role=user, snooze_hint
//       - why              → verb=asked_why, role=user
//       - unclear          → NO feedback_event written
//   * Q3.A + Q3.C — keep 0.7 global threshold; add ≤3-word safe rule;
//     LOCKED explicit-feedback-phrase allowlist absorbed into
//     parseReplyDeterministic (LLM never sees the canonical short
//     feedback phrases).
//   * Q4.A — ONLY ignore_sender triggers applyFeedback. NO new consumer
//     arms in v0.5.10. Positive-signal feedback_events for this_mattered /
//     more_like_this are captured ONLY (no memory_signal write); PIL or a
//     future positive-signal phase decides consumption.
//   * Q5.A — silent acknowledgment; NO new outbound iMessage. HMR
//     Feedback Acknowledgment is its own future-phase candidate.
//   * Q6.A-modified — extend `feedback.written` detail with 10 locked
//     fields (intent_source, inbound_reply_id, parser_intent,
//     parser_confidence, source_surface, verb, dimension, role,
//     legacy_kind?, feedback_event_id). NO new audit kind.
//     HARD PRIVACY RULE: no raw reply text / subject / body / snippet /
//     headers / sender_email in new detail.
//
// Forbidden in v0.5.10 (preserves prior phase locks + permanent product
// principles):
//   * PIL ranking consumption of any feedback-derived signal
//   * New applyFeedback consumer arms beyond v0.5.9's
//     (email_alert, ignored, sender) → sender_feedback_ignored
//   * HMR feedback-prompt surface / acknowledgment iMessage
//     (Q5.A locks: silent; HMR Acknowledgment is its own future gate)
//   * Auto-send (own gate per FOMO_PLAN v0.8)
//   * FOMO_AUTO_SEND_ENABLED=true
//   * BREVIO_DEV_MODE=true
//   * New source_surface activation beyond email_alert
//   * Renaming v0.1.0 intents (names stay; v0.5.10 ADDS)
//   * Storing reply text in any column / detail field
//   * STOP/START as preference feedback (deterministic compliance unchanged)
//   * 3E.1 reversal (classifier is reasoning, NOT body composition)
//   * Friend C onboarding (three-friend cap)
//
// The runbook (docs/smoke-test-v0.5.10-reply-parser-feedback.md) covers
// out-of-band requirements preflight cannot check:
//   * Baseline snapshot of recent feedback_events + sender_feedback_ignored
//     + inbound_replies rows captured (so post-smoke diff proves new rows
//     are from the smoke, not prior state)
//   * SendBlue inbound webhook reachable (ngrok recommended; per
//     [[sendblue-plan-gates]] inbound MAY be blocked by tier — curl-substitute
//     pattern documented in §6 Test 1 fallback per [[v05-2-pass]])
//   * No friend involved this phase (three-friend cap)

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
      message: `${name} required for v0.5.10 (${ctx}). Set ≥ ${min} explicitly.`
    });
    return;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || n < min) {
    issues.push({
      name,
      severity: 'error',
      message: `${name}=${raw} is below the v0.5.10 minimum of ${min}. ${ctx}`
    });
  }
}

console.log('Phase v0.5.10 preflight — Reply-parser feedback intents (founder-only smoke)\n');

/* ---------------------------------------------------------------------- */
/* Substrate carry-forward (v0.5.1 through v0.5.9)                        */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required.');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');
requireMin('BREVIO_PHONE_HASH_KEY', 32, 'BREVIO_PHONE_HASH_KEY must be a 32-byte key');
// v0.5.9 invariant: BREVIO_SENDER_HASH_KEY is load-bearing for the existing
// applyFeedback consumer that v0.5.10 also exercises via ignore_sender intent.
requireMin(
  'BREVIO_SENDER_HASH_KEY',
  32,
  'BREVIO_SENDER_HASH_KEY must be a 32-byte key (v0.5.9 substrate carry-forward; reused by the v0.5.10 ignore_sender → sender_feedback_ignored consumer pipe).'
);

expectEquals(
  'FOMO_FRIEND_BETA_ENABLED',
  'true',
  'v0.5.10 substrate still requires the friend-beta kill switch ON (carry-forward; no friend involved this phase but substrate stays live).'
);

require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required (the polled-alert chain that produces real iMessage threads for Test 1 still uses Gmail).');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required.');
require_('BREVIO_OAUTH_REDIRECT_URI_GOOGLE', 'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required.');

// SendBlue env vars are LOAD-BEARING this phase — the SendBlue inbound
// webhook is the natural-reply ingestion point. Per [[v05-2-pass]] +
// [[sendblue-plan-gates]] inbound may be tier-blocked; the runbook §6 Test 1
// fallback uses a signed-curl substitute when inbound is unreachable.
require_('SENDBLUE_API_KEY_ID', 'SENDBLUE_API_KEY_ID required.');
require_('SENDBLUE_API_SECRET_KEY', 'SENDBLUE_API_SECRET_KEY required.');
require_('SENDBLUE_FROM_NUMBER', 'SENDBLUE_FROM_NUMBER required.');
require_('SENDBLUE_WEBHOOK_SECRET', 'SENDBLUE_WEBHOOK_SECRET required (the inbound webhook signature gate set in SendBlue dashboard; used by the route handler AND by the curl-substitute fallback in §6 Test 1 if needed).');

require_('OPENAI_API_KEY', 'OPENAI_API_KEY required (reply-parser-v0.2.0 classifier + the existing ranker continue to run).');
expectEquals('FOMO_RANKER_ENABLED', 'true', 'Ranker must be on — Test 1 needs a real ranked alert to thread the reply against.');
expectEquals(
  'FOMO_SLACK_REVIEW_ENABLED',
  'true',
  'Slack review must be on — the Slack interactivity audit gains intent_source field in v0.5.10 (symmetry change).'
);
expectEquals(
  'FOMO_SEND_ENABLED',
  'true',
  'FOMO_SEND_ENABLED must be true so the outbound worker can produce the alert message Test 1 replies to.'
);
expectEquals(
  'FOMO_GMAIL_POLLING_ENABLED',
  'true',
  'Polling must be on so the alert that Test 1 replies to gets surfaced.'
);
expectEquals(
  'FOMO_SENDBLUE_INBOUND_ENABLED',
  'true',
  'FOMO_SENDBLUE_INBOUND_ENABLED must be true — the inbound webhook is the v0.5.10 ingestion point.'
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
  'v0.5.10 smoke needs the polling worker live long enough for the alert that Test 1 replies to to actually fire.'
);
checkCycleMin(
  'FOMO_OUTBOUND_MAX_CYCLES',
  30,
  'v0.5.10 smoke keeps the outbound worker live so the alert iMessage is actually delivered.'
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
  // v0.5.9 feedback substrate (still required — v0.5.10 EXTENDS detail
  // on feedback.written and EXERCISES the existing brevio.feedback.applied
  // path via the ignore_sender intent)
  'feedback.written',
  'brevio.feedback.applied'
] as const satisfies readonly AuditAction[];
const auditActionSet = new Set<string>(FOMO_AUDIT_ACTIONS as readonly string[]);
const missingCarryForward = requiredCarryForwardActions.filter((a) => !auditActionSet.has(a));
if (missingCarryForward.length > 0) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'error',
    message: `Prior-phase audit actions missing from registry (still required for v0.5.10): ${missingCarryForward.join(', ')}`
  });
}

const memorySignalSet = new Set(MEMORY_SIGNAL_KINDS as readonly string[]);
const requiredCarryForwardMemoryKinds = [
  'stop_active',
  'sendblue_contact_status',
  // v0.5.9 invariant: still required because v0.5.10 ignore_sender intent
  // path triggers an upsert against this kind via the v0.5.9 consumer arm.
  'sender_feedback_ignored'
] as const;
for (const k of requiredCarryForwardMemoryKinds) {
  if (!memorySignalSet.has(k)) {
    issues.push({
      name: 'MEMORY_SIGNAL_KINDS',
      severity: 'error',
      message: `Prior-phase memory_signal kind '${k}' missing from registry (still required for v0.5.10).`
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
      `v0.5.7 HMR carry-forward invariant: FOUNDER_TEXT_TEMPLATE_VERSION must remain '${V057_TEMPLATE_VERSION_BASELINE}'. Current: '${FOUNDER_TEXT_TEMPLATE_VERSION}'. v0.5.10 is reply-parser routing; it MUST NOT touch the HMR template.`
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
    message: `v0.5.9 invariant: BREVIO_FEEDBACK_ACTIVE_SURFACES must be exactly ['email_alert']. Current: [${BREVIO_FEEDBACK_ACTIVE_SURFACES.join(',')}].`
  });
}

// BREVIO_FEEDBACK_EVENT_KINDS still includes the 6 v0.5.9 generic verbs.
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

/* ---------------------------------------------------------------------- */
/* v0.5.10-specific operator gate                                         */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_V0_5_10_BASELINE_CONFIRMED',
  'true',
  'v0.5.10 baseline gate: founder must run the runbook §1 baseline snapshot BEFORE the smoke starts. ' +
    'Snapshot records: existing feedback_events count + sender_feedback_ignored memory_signal count + ' +
    'inbound_replies count + recent feedback.written audit rows (none yet with the new intent_source field). ' +
    'AFTER capture, set FOMO_V0_5_10_BASELINE_CONFIRMED=true and re-run preflight.'
);

/* ---------------------------------------------------------------------- */
/* PENDING runtime commit warnings                                        */
/* ---------------------------------------------------------------------- */

// PROMPT_VERSION bump — runtime commit flips this to 'reply-parser-v0.2.0'.
// At scaffolding time TS narrows REPLY_PARSER_PROMPT_VERSION to the v0.1.0
// literal; cast to string so the comparison compiles before the runtime
// commit changes the literal (mirrors the v0.5.8/v0.5.9 scaffolding pattern).
const EXPECTED_V0510_PROMPT_VERSION = 'reply-parser-v0.2.0' as string;
const currentPromptVersion = REPLY_PARSER_PROMPT_VERSION as string;
if (currentPromptVersion !== EXPECTED_V0510_PROMPT_VERSION) {
  issues.push({
    name: 'REPLY_PARSER_PROMPT_VERSION',
    severity: 'warn',
    message:
      `reply-parser PROMPT_VERSION='${currentPromptVersion}' (expected '${EXPECTED_V0510_PROMPT_VERSION}' after runtime commit). ` +
      `The bump indicates the classifier intent set extended from 6 → 8 (adds this_mattered + more_like_this). ` +
      `After runtime + smoke PASS this warn disappears.`
  });
}

// feedback-routing.ts module presence — runtime commit creates this file.
// At scaffolding time the file does not exist; the dynamic-import path is
// resolved at runtime, but TS still type-checks the import target. We use
// an indirect path string so TS skips resolution at compile time.
let routingModulePresent = false;
try {
  const modulePath = '../src/reply-parser/feedback-routing.js';
  const mod = (await import(modulePath)) as Record<string, unknown>;
  routingModulePresent = typeof mod.routeReplyFeedback === 'function';
} catch {
  routingModulePresent = false;
}
if (!routingModulePresent) {
  issues.push({
    name: 'FEEDBACK_ROUTING_MODULE',
    severity: 'warn',
    message:
      'apps/fomo/src/reply-parser/feedback-routing.ts (Q1.B policy module) PENDING runtime commit. ' +
      'Will export `routeReplyFeedback(input, deps): Promise<RouteOutcome>` with hardcoded match arms for the 8 intents. ' +
      'After runtime + smoke PASS this warn disappears.'
  });
}

// Operator reminder: the runtime commit will also gain a deterministic
// allowlist absorbed into parseReplyDeterministic for the explicit-feedback-
// phrase short forms (Q3.C). No grep target yet; documented for visibility.
issues.push({
  name: 'EXPLICIT_FEEDBACK_PHRASE_ALLOWLIST',
  severity: 'warn',
  message:
    `Q3.C explicit-feedback-phrase allowlist PENDING runtime commit. ` +
    `Will absorb the locked short-feedback canonical phrases (e.g. 'not important', 'this mattered', 'more like this', 'ignore this sender') ` +
    `into parseReplyDeterministic so the LLM classifier never sees them. Each maps to its intent with confidence=1.0. ` +
    `After runtime + smoke PASS this warn disappears.`
});

// Operator reminder: dimension union extension to 'importance' | 'pattern'.
issues.push({
  name: 'DIMENSION_UNION_EXTENSION',
  severity: 'warn',
  message:
    `LegacyMappedFeedback.overlay.dimension union currently ['sender','alert','ranker_label','thread','topic']. ` +
    `v0.5.10 runtime extends to include 'importance' | 'pattern' (for the new positive-signal intents). ` +
    `Additive; existing callers unchanged. After runtime + smoke PASS this warn disappears.`
});

/* ---------------------------------------------------------------------- */
/* Kill-switch sanity (carry-forward of v0.5.x scope-boundary checks)     */
/* ---------------------------------------------------------------------- */

if ((process.env.FOMO_AUTO_SEND_ENABLED ?? '').trim() === 'true') {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message:
      'FOMO_AUTO_SEND_ENABLED=true is forbidden in v0.5.10. Auto-send is its own future gate per FOMO_PLAN v0.8.'
  });
}

if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'error',
    message:
      'BREVIO_DEV_MODE=true skips Postgres persistence (in-memory stores). v0.5.10 smoke MUST run against real Neon Postgres so feedback_events + sender_feedback_ignored writes are observable in the live DB.'
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
console.log('  Next steps (see docs/smoke-test-v0.5.10-reply-parser-feedback.md):');
console.log('    1. §1 baseline snapshot captured (FOMO_V0_5_10_BASELINE_CONFIRMED=true)');
console.log('    2. Boot dev server: pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.10.log');
console.log('    3. Boot ngrok (or use existing tunnel) so SendBlue inbound can reach the dev server');
console.log('    4. Run §6 Test 1 (LOAD-BEARING): founder texts "ignore this sender" to a real alert thread');
console.log('       Assert: feedback_events row (intent_source=reply_parser_deterministic, parser_intent=ignore_sender)');
console.log('       + sender_feedback_ignored memory_signal upsert + brevio.feedback.applied audit');
console.log('       Fallback if SendBlue inbound is tier-blocked: signed-curl substitute (runbook §6 Test 1 fallback)');
console.log('    5. Run §6 Test 2 (positive intent): founder texts "this mattered"');
console.log('       Assert: feedback_events row with verb=approved, dimension=importance, value=confirmed_important');
console.log('       NO memory_signal upsert; NO brevio.feedback.applied audit');
console.log('    6. Run §6 Test 3 (≤3-word safe rule): founder texts "got it" or similar non-allowlist 2-word reply');
console.log('       Assert: forced to unclear; NO feedback_event written');
console.log('    7. Run §6 Test 4 (STOP regression): founder texts STOP');
console.log('       Assert: existing deterministic compliance fires; NO v0.5.10 feedback_event written');
console.log('    8. Run §6 Test 5 (cross-tenant)');
console.log('    9. Run all 10 evidence scripts:');
console.log('       pnpm smoke-evidence:v0.5.1 && ... && pnpm smoke-evidence:v0.5.9 && \\');
console.log('       pnpm smoke-evidence:v0.5.10');
console.log('   10. Fill in docs/SMOKE_REPORT_v0.5.10.md');
