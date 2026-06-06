// Phase v0.5.9 preflight — Feedback + Learn/Grow Loop substrate (Brevio-wide).
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-06):
//   This script is part of the v0.5.9 SCAFFOLDING commit, which lands BEFORE
//   the runtime implementation. v0.5.9 introduces:
//     - new audit kind `brevio.feedback.applied` (consumer-side, fires when
//       feedback triggers a memory_signal upsert)
//     - extended detail on existing `feedback.written` audit (source_surface,
//       verb, dimension, role, legacy_kind — additive, never breaking)
//     - new memory_signal kind `sender_feedback_ignored` (write-only this
//       phase; ranker does NOT consume — PIL is a future phase)
//     - new feedback_events.source_surface column (via migration 0007)
//     - Brevio-wide BREVIO_FEEDBACK_SURFACES enum (13 surfaces declared;
//       only `email_alert` is in BREVIO_FEEDBACK_ACTIVE_SURFACES)
//     - generic event-kind taxonomy + compatibility mapping for the 11 legacy
//       email-shaped kinds (per Q3.A-modified)
//     - new env var BREVIO_SENDER_HASH_KEY (32 bytes — HMAC key for the
//       privacy-preserving scope_key derivation in memory_signal writes)
//
//   These are EXPECTED OUTPUTS of the future runtime commit. While the
//   runtime commit is pending, the audit kind and memory_signal kind will
//   be absent from FOMO_AUDIT_ACTIONS / MEMORY_SIGNAL_KINDS respectively.
//   Both are reported as PENDING runtime commit (severity 'warn', exit
//   code 0). Once the runtime commit lands and registers both, the warns
//   disappear.
//
// Pure config inspection — no DB, no network. Validates that:
//   (a) the substrate is the v0.5.8-PASS shape (so the v0.5.9 smoke runs
//       against a known-good substrate)
//   (b) the v0.5.9-specific founder gate is set (FOMO_V0_5_9_BASELINE_CONFIRMED)
//   (c) the new privacy-preserving sender-hash key is present and ≥32 bytes
//
// v0.5.9 scope (locked Q1–Q6 — see memory project_v05-9-scope):
//   * Q1.A-modified — additive ALTER TABLE on feedback_events:
//     ADD COLUMN source_surface text NOT NULL DEFAULT 'email_alert' +
//     index (user_id, source_surface). Keep `alert_id`, `sender_email`,
//     table name. NO renames. NO `source_ref_type/_id` (deferred).
//   * Q2.A — BREVIO_FEEDBACK_SURFACES declares all 13 future surfaces;
//     BREVIO_FEEDBACK_ACTIVE_SURFACES = ['email_alert'] (locked exact).
//     No env-var sprawl; activation is code-level.
//   * Q3.A-modified — generic kinds [approved, rejected, snoozed, ignored,
//     asked_why, corrected] (+ optional `opened` only if a current caller
//     exists). Surface-specific meaning in detail.{dimension, target, role,
//     reason, legacy_kind}. Compatibility map for 10 of 11 legacy kinds
//     (`stop` NOT mapped — consent/control stays separate from preference
//     learning per founder lock).
//   * Q4.C — reply-parser feedback DEFERRED to a future phase. v0.5.9
//     captures from existing Slack interactivity + new ops:feedback-inject
//     CLI (founder-only, local/dev only, refuses NODE_ENV=production
//     without explicit dev override).
//   * Q5.B — ONE concrete consumer pipe ships:
//       (source_surface='email_alert', kind='ignored', detail.dimension='sender')
//       → upsert memory_signals(kind='sender_feedback_ignored', scope_key=<hashed>)
//     Per-user, cross-tenant isolated, reversible, audit-visible (brevio.feedback.applied).
//     No raw email body/subject/snippet in audit or memory_signal detail.
//   * Q6.A + Q6.C — extend feedback.written detail (additive) + new audit kind
//     brevio.feedback.applied (fires per memory_signal upsert).
//
// Privacy guardrails (founder-locked 2026-06-06 at approval time):
//   * memory_signals(kind='sender_feedback_ignored').scope_key MUST be a
//     keyed hash, not a plain sender_email. Locked shape:
//       scope_key = HMAC-SHA-256(
//         BREVIO_SENDER_HASH_KEY,
//         user_id + ':' + email.trim().toLowerCase()
//       )
//       hex-encoded, truncated to 32 hex chars (128 bits).
//     The user_id participation in the MAC input prevents cross-user
//     enumeration even with key compromise. Per-user-isolated by construction.
//   * brevio.feedback.applied audit detail MUST NOT include raw sender_email
//     — uses memory_signal_scope_key_hash (the same hashed value).
//   * The existing feedback_events.sender_email column STAYS as legacy v0.5.x
//     state (this scaffolding does NOT expand raw-email leakage to new
//     audit/memory_signal detail; it also does NOT reduce existing storage).
//   * ops:feedback-inject script accepts plain `--sender <email>` as input,
//     normalizes + hashes BEFORE writing to memory_signals/audit. The legacy
//     feedback_events.sender_email column receives the plain value via
//     existing v0.5.x conventions.
//
// Forbidden in v0.5.9 (preserves prior phase locks + permanent product
// principles):
//   * HMR / renderer / ranker prompt changes — own future phase (v0.5.7 PASS
//     state preserved; no HMR feedback-prompt surface this phase)
//   * Reply-parser feedback intent parsing — Q4.C deferred
//   * PIL ranking behavior — ranker does NOT consume sender_feedback_ignored
//     this phase
//   * LLM body generation — 3E.1 directive 2026-05-25 PRESERVED. v0.5.9 is
//     substrate; no LLM call introduced.
//   * FOMO_AUTO_SEND_ENABLED=true
//   * BREVIO_DEV_MODE=true
//   * STOP/START treated as preference feedback — consent/control stays
//     separate from preference learning (founder lock)
//   * SendBlue tier work (F1) — own future phase
//   * Friend C onboarding — three-friend cap holds; founder-only smoke
//   * Dashboard / web UI
//   * Raw email content (subject, body, snippet) in any new audit/memory
//     detail
//   * `brevio_feedback_events` table creation — Q1.A-modified locks: no
//     new table; additive ALTER on existing feedback_events
//   * Sender_email column rename — Q1.A-modified locks: NO rename in v0.5.9
//   * `source_ref_type` / `source_ref_id` columns — locked DEFER; do NOT
//     add unless a future non-email surface activates and needs them
//
// The runbook (docs/smoke-test-v0.5.9-feedback-substrate.md) covers
// out-of-band requirements preflight cannot check:
//   * Baseline snapshot of recent `feedback_events` + `memory_signals`
//     rows captured (so the post-smoke diff proves new rows are from
//     the smoke, not prior state)
//   * Migration 0007_feedback_events_source_surface.sql applied to the
//     live DB (founder runs psql; preflight WARNs at runtime time if
//     the column is missing — smoke-evidence FAILS C2 with the same shape)
//   * ops:feedback-inject script available (lands in runtime commit;
//     scaffolding script entry already in package.json)
//   * No friend involved this phase (three-friend cap holds)

import { loadKillSwitches } from '../src/core/kill-switches.js';
import { FOMO_AUDIT_ACTIONS, type AuditAction } from '../src/core/audit.js';
import { MEMORY_SIGNAL_KINDS } from '../src/memory/memory-signals.js';
import { FOUNDER_TEXT_TEMPLATE_VERSION } from '../src/core/founder-text-template.js';

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
      message: `${name} required for v0.5.9 (${ctx}). Set ≥ ${min} explicitly.`
    });
    return;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || n < min) {
    issues.push({
      name,
      severity: 'error',
      message: `${name}=${raw} is below the v0.5.9 minimum of ${min}. ${ctx}`
    });
  }
}

console.log('Phase v0.5.9 preflight — Feedback + Learn/Grow Loop substrate (founder-only smoke)\n');

/* ---------------------------------------------------------------------- */
/* Substrate carry-forward (v0.5.1 through v0.5.8)                        */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required.');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');
requireMin('BREVIO_PHONE_HASH_KEY', 32, 'BREVIO_PHONE_HASH_KEY must be a 32-byte key');

// NEW in v0.5.9 — privacy-preserving sender_email hashing key (mirrors the
// existing BREVIO_PHONE_HASH_KEY pattern). Used by the consumer-side
// memory_signal upsert to derive a per-user, non-reversible scope_key for
// `sender_feedback_ignored`. NEVER reused for any other purpose.
requireMin(
  'BREVIO_SENDER_HASH_KEY',
  32,
  'BREVIO_SENDER_HASH_KEY must be a 32-byte key (HMAC-SHA-256 key for the v0.5.9 sender scope_key derivation). ' +
    'Generate with: `openssl rand -base64 32`. NEVER reuse from BREVIO_TOKEN_KEK / BREVIO_PHONE_HASH_KEY / any other key (separate hash domain).'
);

expectEquals(
  'FOMO_FRIEND_BETA_ENABLED',
  'true',
  'v0.5.9 substrate still requires the friend-beta kill switch ON (carry-forward; no friend involved this phase but substrate stays live).'
);

require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required (Slack interactivity feedback path needs the polled-alert chain alive).');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required.');
require_(
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE',
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required.'
);

// SendBlue env vars stay required because the substrate stays live — v0.5.9
// is feedback substrate, NOT a delivery change. The outbound worker continues
// to call SendBlue and records audits.
require_('SENDBLUE_API_KEY_ID', 'SENDBLUE_API_KEY_ID required (substrate continues; not load-bearing for v0.5.9 PASS).');
require_('SENDBLUE_API_SECRET_KEY', 'SENDBLUE_API_SECRET_KEY required.');
require_('SENDBLUE_FROM_NUMBER', 'SENDBLUE_FROM_NUMBER required.');

require_('OPENAI_API_KEY', 'OPENAI_API_KEY required (ranker continues on v0.5.7 prompt; v0.5.9 does NOT change prompt and does NOT consume sender_feedback_ignored).');
expectEquals('FOMO_RANKER_ENABLED', 'true', 'Ranker must be on — v0.5.9 Test 2 (Slack interactivity regression) exercises the full ranked-alert chain.');
expectEquals(
  'FOMO_SLACK_REVIEW_ENABLED',
  'true',
  'Slack review must be on — Test 2 proves the existing Slack-interactivity path still writes feedback_events with the new source_surface field populated.'
);
expectEquals(
  'FOMO_SEND_ENABLED',
  'true',
  'FOMO_SEND_ENABLED must be true so the outbound worker can be exercised; delivery is opportunistic.'
);
expectEquals(
  'FOMO_GMAIL_POLLING_ENABLED',
  'true',
  'FOMO_GMAIL_POLLING_ENABLED must be true so the Test 2 regression chain (poll → rank → Slack card → approval → feedback_event) actually runs.'
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
      'FOMO_FOUNDER_PHONE_NUMBER required (substrate continues). Real iMessage delivery is NOT load-bearing for v0.5.9 (which is feedback substrate).'
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
      'FOMO_FRIEND_BETA_BASE_URL required (substrate live). HTTPS URL needed for SendBlue inbound webhook even though v0.5.9 does not exercise that path.'
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
  'v0.5.9 smoke needs the polling worker live long enough for Test 2 (founder approves a freshly-ranked Slack card) to fire end-to-end.'
);
checkCycleMin(
  'FOMO_OUTBOUND_MAX_CYCLES',
  30,
  'v0.5.9 smoke keeps the outbound worker live; not load-bearing for v0.5.9 PASS (which proves feedback substrate, not delivery).'
);

/* ---------------------------------------------------------------------- */
/* Prior-phase audit + memory-signal registry invariants                  */
/* ---------------------------------------------------------------------- */

// Strictly typed via `as const satisfies readonly AuditAction[]`. If anyone
// removes one of these from the AuditAction union in a future PR, tsc fails
// here — the same guardrail the founder asked for in v0.5.5.
//
// Note: `feedback.written` is in the AuditAction union (apps/fomo/src/core/
// audit.ts) but historically NOT in FOMO_AUDIT_ACTIONS (which scopes to the
// fomo.* + brevio.* namespaced kinds the runtime registry iterates). The
// v0.5.9 runtime commit SHOULD add `feedback.written` to FOMO_AUDIT_ACTIONS
// since v0.5.9 extends its detail; until then, the membership check below
// uses a widened Set<string> so the legacy kind can be required without
// forcing scaffolding to enforce the registry-add.
const requiredCarryForwardActions = [
  // v0.5.3 hardening (still required)
  'fomo.sendblue.contact_registered',
  'fomo.sendblue.contact_registration_failed',
  'fomo.send.contact_not_registered',
  'fomo.oauth.refreshed',
  'fomo.oauth.refresh_failed',
  'fomo.db.connection_error',
  'fomo.sendblue.delivery_gap_detected',
  // v0.5.5 STOP enforcement (still required)
  'fomo.sendblue.stop_confirmation_sent',
  'fomo.sendblue.stop_confirmation_failed',
  'fomo.alert.suppressed_stop_active',
  'fomo.poll.skipped_stop_active',
  // v0.5.6 schema-violation fallback (still required)
  'fomo.alert.drafter_schema_failed',
  // v0.5.7 HMR degradation (still required — v0.5.9 must NOT regress HMR)
  'fomo.alert.hmr_degradation_applied',
  // v0.5.8 Gmail INBOX event reliability (still required — v0.5.9 must NOT
  // regress the labelAdded path)
  'fomo.gmail.poll.event_observed',
  'fomo.gmail.poll.event_skipped'
  // Note: `feedback.written` is in the AuditAction union (v0.5.x legacy) but
  // historically NOT in FOMO_AUDIT_ACTIONS — that gap is addressed by the
  // v0.5.9 runtime commit. The PENDING check below catches it as a WARN at
  // scaffolding time and flips silent once the runtime commit adds it to
  // the runtime array.
] as const satisfies readonly AuditAction[];
// Widened to Set<string> so we can require kinds that are in the union but
// not currently in the FOMO_AUDIT_ACTIONS runtime array (e.g. feedback.written
// pre-v0.5.9-runtime). Runtime-side membership semantics are unchanged.
const auditActionSet = new Set<string>(FOMO_AUDIT_ACTIONS as readonly string[]);
const missingCarryForward = requiredCarryForwardActions.filter((a) => !auditActionSet.has(a));
if (missingCarryForward.length > 0) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'error',
    message: `Prior-phase audit actions missing from registry (still required for v0.5.9): ${missingCarryForward.join(', ')}`
  });
}

const memorySignalSet = new Set(MEMORY_SIGNAL_KINDS as readonly string[]);
const requiredCarryForwardMemoryKinds = [
  'stop_active',
  'sendblue_contact_status'
] as const;
for (const k of requiredCarryForwardMemoryKinds) {
  if (!memorySignalSet.has(k)) {
    issues.push({
      name: 'MEMORY_SIGNAL_KINDS',
      severity: 'error',
      message: `Prior-phase memory_signal kind '${k}' missing from registry (still required for v0.5.9).`
    });
  }
}

// v0.5.7 HMR carry-forward: FOUNDER_TEXT_TEMPLATE_VERSION must already be
// at 'human-message-v0.3.0'. v0.5.9 is substrate; MUST NOT touch the HMR
// template (or ship any HMR feedback-prompt surface per scope).
const V057_TEMPLATE_VERSION_BASELINE = 'human-message-v0.3.0';
if (FOUNDER_TEXT_TEMPLATE_VERSION !== V057_TEMPLATE_VERSION_BASELINE) {
  issues.push({
    name: 'FOUNDER_TEXT_TEMPLATE_VERSION',
    severity: 'error',
    message:
      `v0.5.7 HMR carry-forward invariant: FOUNDER_TEXT_TEMPLATE_VERSION must remain '${V057_TEMPLATE_VERSION_BASELINE}'. Current: '${FOUNDER_TEXT_TEMPLATE_VERSION}'. v0.5.9 is feedback substrate; it MUST NOT touch the HMR template.`
  });
}

/* ---------------------------------------------------------------------- */
/* v0.5.9-specific operator gate                                          */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_V0_5_9_BASELINE_CONFIRMED',
  'true',
  'v0.5.9 baseline gate: founder must run the runbook §1 baseline snapshot BEFORE the smoke starts. ' +
    'Snapshot records: existing feedback_events row count + memory_signals(kind=sender_feedback_ignored) baseline (should be 0 pre-smoke) + ' +
    'audit_log brevio.feedback.applied baseline (should be 0 pre-smoke; the kind did not exist before this phase). ' +
    'AFTER capture, set FOMO_V0_5_9_BASELINE_CONFIRMED=true and re-run preflight.'
);

/* ---------------------------------------------------------------------- */
/* PENDING runtime commit warnings                                        */
/* ---------------------------------------------------------------------- */

// New audit kind brevio.feedback.applied — registered by runtime commit.
const EXPECTED_V059_APPLIED_KIND = 'brevio.feedback.applied' as string;
const auditStringSet = new Set<string>(FOMO_AUDIT_ACTIONS as readonly string[]);
if (!auditStringSet.has(EXPECTED_V059_APPLIED_KIND)) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'warn',
    message:
      `'${EXPECTED_V059_APPLIED_KIND}' PENDING runtime commit. Will land when apps/fomo/src/core/audit.ts adds the kind to AuditAction union + FOMO_AUDIT_ACTIONS array. After runtime + smoke PASS this warn disappears.`
  });
}

// `feedback.written` — already in AuditAction union (v0.5.x legacy) but NOT
// in FOMO_AUDIT_ACTIONS runtime array. v0.5.9 runtime commit ALSO adds it
// to the runtime array so smoke-evidence iteration sees it. WARN at
// scaffolding time; silent once runtime lands.
const EXPECTED_LEGACY_FEEDBACK_KIND = 'feedback.written' as string;
if (!auditStringSet.has(EXPECTED_LEGACY_FEEDBACK_KIND)) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'warn',
    message:
      `'${EXPECTED_LEGACY_FEEDBACK_KIND}' is in the AuditAction union but NOT in FOMO_AUDIT_ACTIONS (historical gap). v0.5.9 runtime commit adds it so the v0.5.9 evidence script can iterate the registry. After runtime + smoke PASS this warn disappears.`
  });
}

// New memory_signal kind sender_feedback_ignored — registered by runtime commit.
const EXPECTED_V059_SIGNAL_KIND = 'sender_feedback_ignored' as string;
const memoryStringSet = new Set<string>(MEMORY_SIGNAL_KINDS as readonly string[]);
if (!memoryStringSet.has(EXPECTED_V059_SIGNAL_KIND)) {
  issues.push({
    name: 'MEMORY_SIGNAL_KINDS',
    severity: 'warn',
    message:
      `'${EXPECTED_V059_SIGNAL_KIND}' PENDING runtime commit. Will land when apps/fomo/src/memory/memory-signals.ts adds the kind to MEMORY_SIGNAL_KINDS. After runtime + smoke PASS this warn disappears. Write-only this phase — ranker does NOT consume; PIL is a future phase.`
  });
}

// BREVIO_FEEDBACK_SURFACES + BREVIO_FEEDBACK_ACTIVE_SURFACES enums — registered by runtime commit.
// Scaffolding can't import a constant that doesn't exist yet; smoke-evidence
// inspects them at runtime check time. Scaffolding just notes the contract.
issues.push({
  name: 'BREVIO_FEEDBACK_SURFACES',
  severity: 'warn',
  message:
    `BREVIO_FEEDBACK_SURFACES enum (13 surfaces) + BREVIO_FEEDBACK_ACTIVE_SURFACES allowlist (= ['email_alert']) PENDING runtime commit. ` +
    `Locked list: email_alert, calendar_reminder, draft_suggestion, task_update, stock_watch, coffee_routine, travel_signal, tool_result, ` +
    `browser_summary, booking_preparation, payment_preparation, memory_explanation, why_answer. After runtime lands, smoke-evidence C3 verifies exact contents.`
});

// Migration 0007_feedback_events_source_surface.sql — applied by founder
// AFTER runtime + tests are green, NOT by scaffolding. Smoke-evidence C2
// runs the actual DB column check. Scaffolding-time check is just a
// reminder that the file exists.
issues.push({
  name: 'MIGRATION_0007',
  severity: 'warn',
  message:
    `Migration 'apps/fomo/src/db/migrations/0007_feedback_events_source_surface.sql' authored in scaffolding commit. NOT auto-applied. ` +
    `Founder applies via psql AFTER runtime + tests are green: ` +
    `\`psql "$DATABASE_URL" -f apps/fomo/src/db/migrations/0007_feedback_events_source_surface.sql\`. ` +
    `smoke-evidence C2 verifies the column exists in the live DB at smoke time.`
});

// ops:feedback-inject script — lands in runtime commit (it's runtime code
// that depends on the new active-surface gate). Scaffolding just adds the
// package.json script entry pointing to a path that doesn't exist yet.
issues.push({
  name: 'OPS_FEEDBACK_INJECT',
  severity: 'warn',
  message:
    `\`pnpm --filter @brevio/fomo run ops:feedback-inject\` script entry added in scaffolding (package.json). Underlying script ` +
    `apps/fomo/scripts/ops-feedback-inject.ts lands in the runtime commit. The script is LOCAL/DEV-ONLY per founder lock: refuses to ` +
    `run when NODE_ENV=production unless explicit dev override env var is set. No HTTP route. No public admin endpoint. CLI tool pattern ` +
    `mirrors ops:refresh-oauth + diagnose:gmail-history.`
});

/* ---------------------------------------------------------------------- */
/* Kill-switch sanity (carry-forward of v0.5.x scope-boundary checks)     */
/* ---------------------------------------------------------------------- */

// FOMO_AUTO_SEND_ENABLED must NOT be true — v0.5.9 doesn't change send
// approval flow, and the founder lock from FOMO_PLAN v0.8 stays.
if ((process.env.FOMO_AUTO_SEND_ENABLED ?? '').trim() === 'true') {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message:
      'FOMO_AUTO_SEND_ENABLED=true is forbidden in v0.5.9. Auto-send is its own future gate per FOMO_PLAN v0.8.'
  });
}

// BREVIO_DEV_MODE must NOT be true — smoke runs against real Postgres.
if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'error',
    message:
      'BREVIO_DEV_MODE=true skips Postgres persistence (in-memory stores). v0.5.9 smoke MUST run against real Neon Postgres so feedback_events.source_surface + memory_signals.sender_feedback_ignored writes are observable in the live DB.'
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
console.log('  Next steps (see docs/smoke-test-v0.5.9-feedback-substrate.md):');
console.log('    1. §1 baseline snapshot captured (FOMO_V0_5_9_BASELINE_CONFIRMED=true)');
console.log('    2. Apply migration 0007 to Neon:');
console.log('       psql "$DATABASE_URL" -f apps/fomo/src/db/migrations/0007_feedback_events_source_surface.sql');
console.log('    3. Boot dev server: pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.9.log');
console.log('    4. Run §6 Test 1 (Path A LOAD-BEARING):');
console.log('       pnpm --filter @brevio/fomo run ops:feedback-inject -- \\');
console.log('         --user-id founder \\');
console.log('         --kind ignored \\');
console.log('         --source-surface email_alert \\');
console.log('         --dimension sender \\');
console.log('         --sender <synthetic-email>');
console.log('       Assert: feedback_events row + memory_signals(kind=sender_feedback_ignored)');
console.log('       row + feedback.written audit (extended detail) + brevio.feedback.applied audit');
console.log('    5. Run §6 Test 2 (Slack interactivity regression — approve a real Slack card)');
console.log('    6. Run §6 Test 3 (HMR regression: smoke-evidence:v0.5.7 still PASSES)');
console.log('    7. Run §6 Test 4 (cross-tenant — non-founder rows untouched)');
console.log('    8. Run §6 Test 5 (active-surface live reject — calendar_reminder write fails)');
console.log('    9. Run all 9 evidence scripts:');
console.log('       pnpm smoke-evidence:v0.5.1 && pnpm smoke-evidence:v0.5.2 && \\');
console.log('       pnpm smoke-evidence:v0.5.3 && pnpm smoke-evidence:v0.5.4 && \\');
console.log('       pnpm smoke-evidence:v0.5.5 && pnpm smoke-evidence:v0.5.6 && \\');
console.log('       pnpm smoke-evidence:v0.5.7 && pnpm smoke-evidence:v0.5.8 && \\');
console.log('       pnpm smoke-evidence:v0.5.9');
console.log('   10. Fill in docs/SMOKE_REPORT_v0.5.9.md');
