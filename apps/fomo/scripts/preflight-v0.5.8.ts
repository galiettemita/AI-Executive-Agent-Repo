// Phase v0.5.8 preflight — Gmail INBOX Event Reliability Hardening (founder-only smoke).
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-06):
//   This script is part of the v0.5.8 SCAFFOLDING commit, which lands BEFORE
//   the runtime implementation. v0.5.8 introduces TWO new audit kinds:
//     - fomo.gmail.poll.event_observed   (per-(cycle, message_id) structural observability)
//     - fomo.gmail.poll.event_skipped    (Q5 malformed-labelAdded fallback)
//   AND adds four NEW cycle-level aggregate counters to the existing
//   `gmail.poll.cycle` audit detail:
//     - messages_observed_via_messageAdded_only
//     - messages_observed_via_labelAdded_only        (key metric — counts v0.5.7-era misses)
//     - messages_observed_via_both
//     - messages_dedupe_drops
//   AND swaps `historyTypes='messageAdded'` to `historyTypes='messageAdded,labelAdded'`
//   in `apps/fomo/src/adapters/gmail/client.ts:listHistorySince` + post-filters
//   `labelAdded` events to those where the added labels include the literal
//   string `'INBOX'` (Q2.A lock).
//
//   These are EXPECTED OUTPUTS of the future runtime commit, not
//   already-existing behaviour. While the runtime commit is pending, the two
//   audit kinds will be absent from FOMO_AUDIT_ACTIONS. Both are reported as
//   PENDING runtime commit (severity 'warn', exit code 0). Once the runtime
//   commit lands and registers both, the warns disappear.
//
// Pure config inspection — no DB, no network. Validates that the substrate
// is the v0.5.7-PASS shape (so the v0.5.8 smoke runs against a known-good
// substrate) AND that the v0.5.8-specific founder gate is set.
//
// v0.5.8 scope (locked Q1–Q6 — see memory project_v05-8-scope):
//   * Q1.A — single Gmail history.list call, comma-separated:
//     `historyTypes='messageAdded,labelAdded'`. Smallest diff; preserves
//     cursor advancement semantics exactly.
//   * Q2.A — accept `labelAdded` events ONLY where the added labels include
//     the literal string `'INBOX'` (a reserved Gmail system label; no
//     per-user lookup).
//   * Q3.A — per-cycle in-memory `Set<message_id>` populated as we iterate
//     history items. On second sighting of the same `message_id`
//     (regardless of which event type triggered), skip the dispatch.
//     First-seen wins. The existing `rank_results.UNIQUE(user_id, message_id)`
//     is the load-bearing fallback if any seen-set race ever drops one.
//   * Q4.A — trust existing `rank_results.UNIQUE(user_id, message_id)` for
//     cross-cycle dedupe. NO new persistence layer; the in-memory per-cycle
//     dedupe (Q3.A) plus the DB UNIQUE catches all race shapes.
//   * Q5 — locked degradation matrix:
//       - labelAdded missing `addedLabels` (malformed) → skip event silently;
//         emit `fomo.gmail.poll.event_skipped` (best-effort, NO retry)
//       - labelAdded NOT 'INBOX' (STARRED/custom/etc.) → ignore silently, no audit
//       - INBOX added then removed in same cursor span → process (let user STOP)
//       - history.list 5xx / 404 cursor expired → existing retry/re-bootstrap (no change)
//       - same message in BOTH event types same cycle → Q3.A dedupe; ONE dispatch
//   * Q6.A with restraint — TWO observability artifacts:
//       1. NEW audit kind `fomo.gmail.poll.event_observed` — fired once per
//          (cycle, message_id) AFTER dedupe. STRUCTURAL FIELDS ONLY:
//            - event_types_seen: ('messageAdded'|'labelAdded')[]
//            - inbox_label_present: boolean
//            - is_dedupe_drop: boolean
//            - message_id: Gmail message id
//          NEVER subject/sender/body/raw label names beyond the boolean
//          derivative/raw event JSON/attachment names.
//       2. NEW cycle-level counters on `gmail.poll.cycle` detail (cheaper
//          observability than scanning per-message rows):
//            - messages_observed_via_messageAdded_only
//            - messages_observed_via_labelAdded_only   ← KEY METRIC
//            - messages_observed_via_both
//            - messages_dedupe_drops
//          The existing `messages_observed` counter continues to count
//          post-dedupe UNIQUE messages (documented invariant).
//
// Forbidden in v0.5.8 (preserves prior phase locks + permanent product
// principles):
//   * HMR / renderer / ranker prompt changes — own future phase (v0.5.7 PASS
//     state preserved)
//   * LLM body generation — 3E.1 directive 2026-05-25 PRESERVED. v0.5.8 is
//     poller-layer only; no LLM call introduced.
//   * Personalized Importance Learning substrate — own phase
//   * Feedback + Learn/Grow Loop substrate — strategic next-phase candidate;
//     its own 6Q gate
//   * FOMO_AUTO_SEND_ENABLED=true
//   * BREVIO_DEV_MODE=true
//   * SendBlue tier work (F1) — own future phase
//   * Friend C onboarding — three-friend cap; expansion is its own decision
//   * Dashboard / web UI
//   * Raw email content in new audit detail — Q6 lock: structural enums +
//     booleans + message_id only
//   * Schema / table / migration (Q4.A locks NO new persistence)
//
// The runbook (docs/smoke-test-v0.5.8-gmail-inbox-reliability.md) covers
// out-of-band requirements preflight cannot check:
//   * Baseline snapshot of recent `gmail.poll.cycle` rows captured (so we
//     can prove v0.5.7-era zero-counter shape vs v0.5.8 ≥1 labelAdded_only)
//   * Path A default: Gmail-to-self synthetic important email is load-bearing
//     proof (v0.5.7 baseline = NEVER surfaces; v0.5.8 baseline = ≤3 cycles).
//   * External regression secondary
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
      message: `${name} required for v0.5.8 (${ctx}). Set ≥ ${min} explicitly.`
    });
    return;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || n < min) {
    issues.push({
      name,
      severity: 'error',
      message: `${name}=${raw} is below the v0.5.8 minimum of ${min}. ${ctx}`
    });
  }
}

console.log('Phase v0.5.8 preflight — Gmail INBOX Event Reliability Hardening (founder-only smoke)\n');

/* ---------------------------------------------------------------------- */
/* Substrate carry-forward (v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4 + v0.5.5    */
/*                          + v0.5.6 + v0.5.7)                            */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required.');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');
requireMin('BREVIO_PHONE_HASH_KEY', 32, 'BREVIO_PHONE_HASH_KEY must be a 32-byte key');

expectEquals(
  'FOMO_FRIEND_BETA_ENABLED',
  'true',
  'v0.5.8 substrate still requires the friend-beta kill switch ON (carry-forward; no friend involved this phase but substrate stays live).'
);

require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required (founder Gmail polling is the load-bearing surface this phase).');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required.');
require_(
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE',
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required.'
);

// SendBlue env vars are STILL required because the substrate stays live —
// even if SendBlue blocks delivery, the outbound worker calls SendBlue and
// records the audit. v0.5.8 is poller-layer; iMessage delivery is NOT
// load-bearing for C10/C11 (which prove rank-side behavior).
require_('SENDBLUE_API_KEY_ID', 'SENDBLUE_API_KEY_ID required (substrate continues; not load-bearing for v0.5.8 PASS).');
require_('SENDBLUE_API_SECRET_KEY', 'SENDBLUE_API_SECRET_KEY required.');
require_('SENDBLUE_FROM_NUMBER', 'SENDBLUE_FROM_NUMBER required.');

require_('OPENAI_API_KEY', 'OPENAI_API_KEY required (ranker continues to run on v0.5.7 prompt; v0.5.8 does NOT change prompt).');
expectEquals('FOMO_RANKER_ENABLED', 'true', 'Ranker must be on — v0.5.8 proof is that the labelAdded-only path produces a rank (≤3 cycles).');
expectEquals(
  'FOMO_SLACK_REVIEW_ENABLED',
  'true',
  'Slack review must be on — v0.5.8 proves the Gmail-to-self synthetic surfaces a Slack card (which v0.5.7 NEVER did).'
);
expectEquals(
  'FOMO_SEND_ENABLED',
  'true',
  'FOMO_SEND_ENABLED must be true so the outbound worker can be exercised (delivery is opportunistic; C10 is the rank, not the send).'
);
expectEquals(
  'FOMO_GMAIL_POLLING_ENABLED',
  'true',
  'FOMO_GMAIL_POLLING_ENABLED must be true so the new labelAdded filter actually runs in the smoke.'
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
      'FOMO_FOUNDER_PHONE_NUMBER required (substrate continues). Real iMessage delivery is NOT load-bearing for v0.5.8 (which is poller-layer); C10 proves rank-side detection.'
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
      'FOMO_FRIEND_BETA_BASE_URL required (substrate live). HTTPS URL needed for SendBlue inbound webhook even though v0.5.8 doesn’t exercise that path.'
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
  'v0.5.8 smoke needs the polling worker live long enough to observe the labelAdded-only path AND prove ≤3-cycle dispatch on the Gmail-to-self synthetic.'
);
checkCycleMin(
  'FOMO_OUTBOUND_MAX_CYCLES',
  30,
  'v0.5.8 smoke keeps the outbound worker live so an approved alert can attempt SendBlue (delivery opportunistic — C10 is rank-side).'
);

/* ---------------------------------------------------------------------- */
/* v0.5.3 + v0.5.5 + v0.5.6 + v0.5.7 audit registry invariants            */
/* ---------------------------------------------------------------------- */

// Strictly typed via `as const satisfies readonly AuditAction[]`. If anyone
// removes one of these from FOMO_AUDIT_ACTIONS in a future PR, tsc fails
// here — the same guardrail the founder asked for in v0.5.5.
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
  // v0.5.7 HMR degradation (still required — v0.5.8 must NOT regress HMR)
  'fomo.alert.hmr_degradation_applied'
] as const satisfies readonly AuditAction[];
const auditActionSet = new Set(FOMO_AUDIT_ACTIONS);
const missingCarryForward = requiredCarryForwardActions.filter((a) => !auditActionSet.has(a));
if (missingCarryForward.length > 0) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'error',
    message: `Prior-phase audit actions missing from registry (still required for v0.5.8): ${missingCarryForward.join(', ')}`
  });
}

const memorySignalSet = new Set(MEMORY_SIGNAL_KINDS as readonly string[]);
if (!memorySignalSet.has('stop_active')) {
  issues.push({
    name: 'MEMORY_SIGNAL_KINDS',
    severity: 'error',
    message:
      "v0.5.5 invariant carried into v0.5.8: 'stop_active' must stay registered."
  });
}

// v0.5.7 HMR carry-forward: FOUNDER_TEXT_TEMPLATE_VERSION must already be
// at 'human-message-v0.3.0'. v0.5.8 does NOT bump it further; we only check
// it hasn't been reverted.
const V057_TEMPLATE_VERSION_BASELINE = 'human-message-v0.3.0';
if (FOUNDER_TEXT_TEMPLATE_VERSION !== V057_TEMPLATE_VERSION_BASELINE) {
  issues.push({
    name: 'FOUNDER_TEXT_TEMPLATE_VERSION',
    severity: 'error',
    message:
      `v0.5.7 HMR carry-forward invariant: FOUNDER_TEXT_TEMPLATE_VERSION must remain '${V057_TEMPLATE_VERSION_BASELINE}'. Current: '${FOUNDER_TEXT_TEMPLATE_VERSION}'. v0.5.8 is a poller-layer hardening; it MUST NOT touch the HMR template.`
  });
}

/* ---------------------------------------------------------------------- */
/* v0.5.8-specific operator gate                                          */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_V0_5_8_BASELINE_CONFIRMED',
  'true',
  'v0.5.8 baseline gate: founder must run the runbook §1 baseline snapshot BEFORE the smoke starts. ' +
    'That snapshot records recent `gmail.poll.cycle` audit rows so v0.5.7-era zero-counter shape (no labelAdded_only counter) can be compared against v0.5.8 ≥1 labelAdded_only readings. ' +
    'It also records the absence of `fomo.gmail.poll.event_observed` rows so the post-smoke presence is provably new. ' +
    'Set FOMO_V0_5_8_BASELINE_CONFIRMED=true only AFTER you have captured the baseline into /tmp/v0.5.8-baseline-gmail-poll-cycle.txt.'
);

const windowHours = (process.env.FOMO_V0_5_8_WINDOW_HOURS ?? '').trim();
if (!windowHours) {
  issues.push({
    name: 'FOMO_V0_5_8_WINDOW_HOURS',
    severity: 'warn',
    message:
      'FOMO_V0_5_8_WINDOW_HOURS not set; smoke-evidence will default to 24h. Override only if the smoke runs across sessions.'
  });
} else {
  const n = Number(windowHours);
  if (!Number.isFinite(n) || n < 1 || n > 168) {
    issues.push({
      name: 'FOMO_V0_5_8_WINDOW_HOURS',
      severity: 'error',
      message: `FOMO_V0_5_8_WINDOW_HOURS=${windowHours} outside 1–168.`
    });
  }
}

/* ---------------------------------------------------------------------- */
/* v0.5.8-NEW expected runtime outputs — PENDING runtime commit           */
/* ---------------------------------------------------------------------- */
/*
 * 1. fomo.gmail.poll.event_observed audit kind — registered by runtime
 *    commit. Fires once per (cycle, message_id) AFTER dedupe. Detail is
 *    STRUCTURAL ONLY: event_types_seen[], inbox_label_present, is_dedupe_drop,
 *    message_id. NEVER raw content.
 *
 * 2. fomo.gmail.poll.event_skipped audit kind — registered by runtime
 *    commit. Q5 fallback: fires when a `labelAdded` event arrives with no
 *    `addedLabels` field (Gmail malformed). Detail: reason='malformed_labelAdded'.
 *    Best-effort, NO retry.
 *
 * 3. Cycle-level counters on existing `gmail.poll.cycle` detail:
 *    messages_observed_via_messageAdded_only, *_labelAdded_only,
 *    *_both, messages_dedupe_drops. Inspected via DB query at smoke-evidence
 *    time, NOT here.
 *
 * 4. apps/fomo/src/adapters/gmail/client.ts:listHistorySince swaps
 *    historyTypes='messageAdded' to historyTypes='messageAdded,labelAdded'.
 *    Inspected via grep + unit test, NOT here.
 *
 * NOTE: items 1 + 2 are checked below (preflight reads the runtime constant).
 * Items 3 + 4 are checked at smoke-evidence time.
 */

// v0.5.8 NEW audit kinds. Until the runtime commit lands, neither is in the
// AuditAction union, so we must use a string cast that survives compile.
// After the runtime commit lands and the union includes them, replace with
// `as const satisfies AuditAction` per the v0.5.5 founder directive.
const EXPECTED_V058_EVENT_OBSERVED_KIND = 'fomo.gmail.poll.event_observed' as string;
const EXPECTED_V058_EVENT_SKIPPED_KIND = 'fomo.gmail.poll.event_skipped' as string;

const auditActionStringSet = new Set<string>(FOMO_AUDIT_ACTIONS as readonly string[]);
if (!auditActionStringSet.has(EXPECTED_V058_EVENT_OBSERVED_KIND)) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'warn',
    message: `v0.5.8 expected audit kind PENDING runtime commit: '${EXPECTED_V058_EVENT_OBSERVED_KIND}'. This is normal at scaffolding time — the kind is registered by the future runtime implementation commit when wiring the Q6.A per-message structural audit.`
  });
}
if (!auditActionStringSet.has(EXPECTED_V058_EVENT_SKIPPED_KIND)) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'warn',
    message: `v0.5.8 expected audit kind PENDING runtime commit: '${EXPECTED_V058_EVENT_SKIPPED_KIND}'. This is normal at scaffolding time — the kind is registered by the future runtime implementation commit when wiring the Q5 malformed-event fallback.`
  });
}

/* ---------------------------------------------------------------------- */
/* 3E.1 directive guardrail — code-level                                  */
/* ---------------------------------------------------------------------- */
/*
 * The 3E.1 directive (2026-05-25) bans LLM body generation. v0.5.8 is a
 * poller-layer hardening; it does NOT touch the body-composition path.
 * There is NO new LLM call introduced by v0.5.8.
 *
 * This comment is the tripwire: if any future v0.5.8 preflight is tempted to
 * add a check like
 *   require_('FOMO_GMAIL_LABEL_CLASSIFIER_MODEL_ID', ...)
 * that would be evidence of reversing 3E.1 — STOP and confirm with founder
 * + update memory feedback_3e1-no-llm-body-generation.md.
 */

/* ---------------------------------------------------------------------- */
/* HMR carry-forward — code-level                                         */
/* ---------------------------------------------------------------------- */
/*
 * v0.5.7 introduced the Human Message Renderer surface. v0.5.8 PRESERVES it
 * by design (poller-layer change only; no renderer / ranker prompt / audit
 * field touched in the outbound-sender path).
 *
 * This comment is the tripwire: if any future v0.5.8 preflight adds a check
 * that mutates HMR runtime constants (e.g. asserting a NEW template_version
 * past 'human-message-v0.3.0'), that would mean v0.5.8 scope creeped into
 * HMR — STOP and surface to founder; v0.5.8 is poller-only.
 */

/* ---------------------------------------------------------------------- */
/* Forbidden in v0.5.8                                                    */
/* ---------------------------------------------------------------------- */

const switches = loadKillSwitches(process.env);
if (switches.auto_send_enabled) {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message:
      'v0.5.8 hard boundary: founder Slack review still required for FOMO alerts. Set FOMO_AUTO_SEND_ENABLED=false (or unset). Auto-send is its own future 6Q gate per FOMO_PLAN v0.8.'
  });
}
if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'error',
    message:
      'BREVIO_DEV_MODE=true is a hard error in v0.5.8 — ephemeral per-process keys would invalidate the founder OAuth tokens between restarts.'
  });
}

console.log('Resolved kill switches:');
console.log(JSON.stringify(switches, null, 2));
console.log('');

/* ---------------------------------------------------------------------- */
/* Report                                                                 */
/* ---------------------------------------------------------------------- */

const errors = issues.filter((i) => i.severity === 'error');
const warns = issues.filter((i) => i.severity === 'warn');

if (errors.length === 0) {
  if (warns.length > 0) {
    console.log(`! ${warns.length} warning(s) (non-blocking):\n`);
    for (const w of warns) console.log(`  [WARN]  ${w.name}: ${w.message}`);
    console.log('');
  }
  console.log('✓ Preflight passed.');
  console.log('');
  console.log('  Next steps (see docs/smoke-test-v0.5.8-gmail-inbox-reliability.md):');
  console.log('    1. §1 baseline snapshot captured (FOMO_V0_5_8_BASELINE_CONFIRMED=true)');
  console.log('    2. Boot dev server: pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.8.log');
  console.log('    3. Run §6 Test 1 (Path A load-bearing: Gmail-to-self synthetic important email; v0.5.7 baseline = NEVER, v0.5.8 baseline = ≤3 cycles; assert fomo.rank.completed + fomo.slack.posted + fomo.gmail.poll.event_observed with event_types_seen containing labelAdded)');
  console.log('    4. Run §6 Test 2 (external regression: icloud/other → gmail external email; messageAdded path still works)');
  console.log('    5. Run §6 Test 3 (HMR regression: smoke-evidence:v0.5.7 still PASSES on this branch — v0.5.8 must NOT touch the renderer)');
  console.log('    6. Run §6 Test 4 (cross-tenant — non-founder rows untouched)');
  console.log('    7. Run all 8 evidence scripts: pnpm smoke-evidence:v0.5.1 && pnpm smoke-evidence:v0.5.2 && pnpm smoke-evidence:v0.5.3 && pnpm smoke-evidence:v0.5.4 && pnpm smoke-evidence:v0.5.5 && pnpm smoke-evidence:v0.5.6 && pnpm smoke-evidence:v0.5.7 && pnpm smoke-evidence:v0.5.8');
  console.log('    8. Fill in docs/SMOKE_REPORT_v0.5.8.md');
  console.log('');
  const pendingCount =
    (auditActionStringSet.has(EXPECTED_V058_EVENT_OBSERVED_KIND) ? 0 : 1) +
    (auditActionStringSet.has(EXPECTED_V058_EVENT_SKIPPED_KIND) ? 0 : 1);
  if (pendingCount > 0) {
    console.log(
      `  NOTE: ${pendingCount} v0.5.8 runtime artifact(s) are PENDING runtime commit. Smoke-evidence will report some criteria as PENDING until the runtime implementation lands. This is expected at scaffolding time.`
    );
    console.log('');
  }
  process.exit(0);
}

console.log(`✖ ${errors.length} required check(s) failed:\n`);
for (const e of errors) console.log(`  [ERROR] ${e.name}: ${e.message}`);
console.log('');
if (warns.length > 0) {
  console.log(`! ${warns.length} warning(s):\n`);
  for (const w of warns) console.log(`  [WARN]  ${w.name}: ${w.message}`);
  console.log('');
}
process.exit(1);
