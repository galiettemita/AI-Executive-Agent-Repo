// Phase v0.5.13 preflight — Founder-only PIL Live Canary / Controlled Activation.
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-07):
//   This script is part of the v0.5.13 SCAFFOLDING commit, which lands BEFORE
//   the runtime implementation. v0.5.13 is the first phase where the v0.5.12
//   live PIL ranker path actually runs against real founder traffic in
//   production, behind a per-user allowlist with reversible env-only rollback.
//
//   Locked scope (see memory project_v05-13-scope):
//     Add `FOMO_PIL_LIVE_USER_ALLOWLIST` env var (comma-separated user_id list).
//     `FOMO_PIL_LIVE_ENABLED=false` stays default.
//     Founder-only allowlist for canary.
//     Per-user gate at the polling-worker call site BEFORE buildLivePilContext.
//
//   4-case truth table (founder-locked):
//     | FOMO_PIL_LIVE_ENABLED | FOMO_PIL_LIVE_USER_ALLOWLIST | Behavior |
//     |----------------------|------------------------------|---------|
//     | false (default)      | (any)                        | Bit-identical v0.5.11 for all users. Allowlist parsed but ignored. |
//     | true                 | unset OR empty               | PREFLIGHT ERRORS at boot. (Runtime fail-closed with WARN log if bypassed.) |
//     | true                 | <founder_user_id>            | Only that user_id gets the v0.5.12 two-call hybrid. Everyone else: baseline-only, no PIL audit. |
//     | true                 | userA,userB                  | Both get hybrid. Contract proves the mechanism is generic. |
//
//   Founder corrections (locked 2026-06-07):
//     1. Allowlist parser TRIMS whitespace only — does NOT lowercase user_ids.
//        Matches existing FOMO_FOUNDER_USER_ID?.trim() convention.
//     2. Preflight ERRORS (not warns) when FOMO_PIL_LIVE_ENABLED=true AND
//        FOMO_PIL_LIVE_USER_ALLOWLIST is empty or unset. Runtime may fail-
//        closed with WARN log, but preflight must be LOUD at the boot gate.
//     3. Final production state after canary: FOMO_PIL_LIVE_ENABLED=false
//        unless founder explicitly approves keeping ON after canary review.
//     4. 24h observation window is NOT a merge blocker. Canary proof can be
//        shorter. Report can RECOMMEND 24h but the PR must not be blocked
//        for a full day if all required proof passes.
//
//   The runtime commit(s) will:
//     - Add `pil_live_user_allowlist: readonly string[]` to KillSwitches
//       (apps/fomo/src/core/kill-switches.ts). Parsed from
//       FOMO_PIL_LIVE_USER_ALLOWLIST as CSV; each entry .trim()ed;
//       NO lowercasing; empty entries filtered out.
//     - Add the per-user gate at apps/fomo/src/workers/gmail-poll.ts: BEFORE
//       calling deps.pilLive.buildLivePilContext(user_id, senderEmailHash),
//       check if user_id is in the allowlist; if not, skip the PIL call and
//       fall through to the baseline-only path (bit-identical to
//       FOMO_PIL_LIVE_ENABLED=false for that user).
//     - Add aggregate counter to the cycle audit:
//       `messages_pil_skipped_not_in_allowlist`.
//     - Unit tests for the 4-case truth table.
//
//   While the runtime commit is pending:
//     - KillSwitches.pil_live_user_allowlist field absent → WARN PENDING
//     - Worker-level allowlist gate absent → WARN PENDING
//
//   When runtime + smoke + report land, all WARNs flip to silent.
//
// Pure config inspection — no DB, no network. Validates that:
//   (a) substrate carry-forward (v0.5.1 through v0.5.12) intact
//   (b) the v0.5.13-specific founder gate is set
//       (FOMO_V0_5_13_BASELINE_CONFIRMED)
//   (c) FOMO_PIL_LIVE_USER_ALLOWLIST is non-empty IFF FOMO_PIL_LIVE_ENABLED=true
//       (LOAD-BEARING ERROR per founder correction #2)
//   (d) v0.5.12 carry-forward invariants (PIL kinds, brevio.rank.pil_applied
//       registration, ranker prompt versions, HMR, reply-parser) still hold
//
// Forbidden in v0.5.13 (preserves prior phase locks + permanent product
// principles):
//   * HMR / renderer / drafter changes (3E.1 invariant carry-forward)
//   * User-facing feedback acknowledgment text (own future gate)
//   * "Why?" answer surface (own future gate)
//   * Auto-send
//   * New tools / new modalities
//   * New active source_surface beyond email_alert
//   * New memory_signal kinds (v0.5.13 is READ-ONLY against v0.5.11 substrate)
//   * Reading legacy scope_key='message:<id>' placeholder rows (v0.5.12 lock)
//   * Raw private content (sender_email, subject, body, snippet, headers) in
//     ranker prompt or audit detail
//   * Global learning / cross-user signal pooling
//   * FOMO_PIL_DIVERGENCE_AUDIT_ENABLED=true by default
//   * Backfill of pre-migration alerts.sender_email_hash NULL rows
//   * FOMO_AUTO_SEND_ENABLED=true
//   * BREVIO_DEV_MODE=true (smoke runs against real Neon)
//   * Friend C onboarding (three-friend cap)
//   * Activating PIL for friends (allowlist stays founder-only this phase)
//
// The runbook (docs/smoke-test-v0.5.13-pil-live-canary.md) covers
// out-of-band requirements preflight cannot check:
//   * Baseline snapshot of brevio.rank.pil_applied count (expected 0 before
//     canary window opens) captured
//   * Soft + hard rollback rehearsal plan (mid-canary env flip timeline)
//   * No friend involved this phase (three-friend cap; founder-only canary)

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
      message: `${name} required for v0.5.13 (${ctx}). Set ≥ ${min} explicitly.`
    });
    return;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || n < min) {
    issues.push({
      name,
      severity: 'error',
      message: `${name}=${raw} is below the v0.5.13 minimum of ${min}. ${ctx}`
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
function expectBoolFalseOrTrue(name: string, ctx: string): void {
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

/**
 * Parse FOMO_PIL_LIVE_USER_ALLOWLIST per founder correction #1:
 *   - Trim whitespace on each entry.
 *   - DO NOT lowercase (preserves exact user_id strings — matches the
 *     existing FOMO_FOUNDER_USER_ID?.trim() convention used throughout
 *     the codebase).
 *   - Filter out empty entries (e.g. trailing comma).
 *   - Return as readonly string[]. Empty input → [].
 *
 * This MUST match the runtime-commit parser semantics exactly. The
 * scaffolding-phase preflight implements it inline to validate the env
 * value without requiring the runtime parser to exist.
 */
function parsePilLiveAllowlist(raw: string | undefined): readonly string[] {
  if (raw === undefined) return [];
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

console.log('Phase v0.5.13 preflight — Founder-only PIL Live Canary / Controlled Activation\n');

/* ---------------------------------------------------------------------- */
/* Substrate carry-forward (v0.5.1 through v0.5.12)                       */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required.');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');
requireMin('BREVIO_PHONE_HASH_KEY', 32, 'BREVIO_PHONE_HASH_KEY must be a 32-byte key');
// LOAD-BEARING for v0.5.13: the live ranker's PIL context lookup HMACs the
// sender_email at read time. The hash MUST match the value the v0.5.11 rank-
// step wrote to alerts.sender_email_hash AND the v0.5.11 aggregation pipe
// wrote to memory_signals.scope_key.
requireMin(
  'BREVIO_SENDER_HASH_KEY',
  32,
  'BREVIO_SENDER_HASH_KEY must be a 32-byte key. v0.5.13 LOAD-BEARING (carry-forward from v0.5.12).'
);

expectEquals(
  'FOMO_FRIEND_BETA_ENABLED',
  'true',
  'v0.5.13 substrate still requires the friend-beta kill switch ON (carry-forward; NO friend involved this phase but substrate stays live).'
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
  'OPENAI_API_KEY required. v0.5.13 LOAD-BEARING — the v0.5.12 two-call hybrid runs TWO ranker calls per PIL-influenced rank.'
);
expectEquals('FOMO_RANKER_ENABLED', 'true', 'Ranker must be on for the canary to exercise the live PIL path.');
expectEquals(
  'FOMO_SLACK_REVIEW_ENABLED',
  'true',
  'Slack review must be on — substrate live for the canary-produced alerts.'
);
expectEquals(
  'FOMO_SEND_ENABLED',
  'true',
  'FOMO_SEND_ENABLED must be true so the outbound worker can deliver canary alerts.'
);
expectEquals(
  'FOMO_GMAIL_POLLING_ENABLED',
  'true',
  'Polling must be on so canary-window ranks happen.'
);
expectEquals(
  'FOMO_SENDBLUE_INBOUND_ENABLED',
  'true',
  'FOMO_SENDBLUE_INBOUND_ENABLED must be true — carry-forward.'
);
require_('SLACK_BOT_TOKEN', 'SLACK_BOT_TOKEN required.');
require_('SLACK_FOUNDER_CHANNEL_ID', 'SLACK_FOUNDER_CHANNEL_ID required.');
require_('SLACK_SIGNING_SECRET', 'SLACK_SIGNING_SECRET required.');
require_(
  'FOMO_FOUNDER_USER_ID',
  'FOMO_FOUNDER_USER_ID required. v0.5.13 LOAD-BEARING — the allowlist must contain THIS exact value for the founder-only canary.'
);

const founderPhone = (process.env.FOMO_FOUNDER_PHONE_NUMBER ?? '').trim();
if (!founderPhone) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message: 'FOMO_FOUNDER_PHONE_NUMBER required (carry-forward).'
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
    message: 'FOMO_FRIEND_BETA_BASE_URL required (substrate live carry-forward).'
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
  'v0.5.13 canary needs the polling worker live long enough for at least one founder rank.'
);
checkCycleMin(
  'FOMO_OUTBOUND_MAX_CYCLES',
  30,
  'v0.5.13 canary keeps the outbound worker live to deliver alert messages.'
);

/* ---------------------------------------------------------------------- */
/* Prior-phase audit + memory-signal registry invariants                  */
/* ---------------------------------------------------------------------- */

const requiredCarryForwardActions = [
  // v0.5.3 hardening
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
  // v0.5.9 feedback substrate
  'feedback.written',
  'brevio.feedback.applied',
  // v0.5.11 PIL substrate write path
  'brevio.signal.aggregated',
  // v0.5.12 PIL applied audit — LOAD-BEARING this phase
  'brevio.rank.pil_applied'
] as const satisfies readonly AuditAction[];
const auditActionSet = new Set<string>(FOMO_AUDIT_ACTIONS as readonly string[]);
const missingCarryForward = requiredCarryForwardActions.filter((a) => !auditActionSet.has(a));
if (missingCarryForward.length > 0) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'error',
    message: `Prior-phase audit actions missing from registry (still required for v0.5.13): ${missingCarryForward.join(', ')}`
  });
}

const memorySignalSet = new Set(MEMORY_SIGNAL_KINDS as readonly string[]);
const requiredCarryForwardMemoryKinds = [
  'stop_active',
  'sendblue_contact_status',
  'sender_feedback_ignored',
  'sender_importance',
  'sender_suppressed'
] as const;
for (const k of requiredCarryForwardMemoryKinds) {
  if (!memorySignalSet.has(k)) {
    issues.push({
      name: 'MEMORY_SIGNAL_KINDS',
      severity: 'error',
      message: `Prior-phase memory_signal kind '${k}' missing from registry (still required for v0.5.13).`
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
      `v0.5.7 HMR carry-forward invariant: FOUNDER_TEXT_TEMPLATE_VERSION must remain '${V057_TEMPLATE_VERSION_BASELINE}'. Current: '${FOUNDER_TEXT_TEMPLATE_VERSION}'. v0.5.13 MUST NOT touch HMR.`
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
    message: `v0.5.9 invariant + v0.5.13 hard boundary: BREVIO_FEEDBACK_ACTIVE_SURFACES must be exactly ['email_alert']. Current: [${BREVIO_FEEDBACK_ACTIVE_SURFACES.join(',')}].`
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
      `v0.5.10 invariant: reply-parser PROMPT_VERSION must remain '${V0510_PROMPT_VERSION_BASELINE}'. Current: '${currentReplyParserVersion}'. v0.5.13 does NOT touch the reply parser.`
  });
}

// v0.5.11 + v0.5.12 substrate modules present — v0.5.13 READS from them.
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
    message: 'apps/fomo/src/memory/pil-aggregation.ts (v0.5.11 applyPilAggregation) MUST be present.'
  });
}

let v0512LivePilContextPresent = false;
try {
  const modulePath = '../src/ranker/pil-context.js';
  const mod = (await import(modulePath)) as Record<string, unknown>;
  v0512LivePilContextPresent = typeof mod.buildLivePilContext === 'function';
} catch {
  v0512LivePilContextPresent = false;
}
if (!v0512LivePilContextPresent) {
  issues.push({
    name: 'V0512_LIVE_PIL_CONTEXT_MODULE',
    severity: 'error',
    message:
      'apps/fomo/src/ranker/pil-context.ts must export buildLivePilContext (v0.5.12 read-side). v0.5.13 reuses this function unchanged; the per-user gate sits in front of it.'
  });
}

/* ---------------------------------------------------------------------- */
/* v0.5.12 tunable carry-forward (FOMO_PIL_LIVE_ENABLED + FOMO_PIL_SCORE_CAP) */
/* ---------------------------------------------------------------------- */

// FOMO_PIL_SCORE_CAP — Q2.A hard cap, bounds [0.05, 0.25], default 0.15.
checkFloatBounds(
  'FOMO_PIL_SCORE_CAP',
  0.15,
  0.05,
  0.25,
  'Q2.A hard cap carry-forward from v0.5.12. Unchanged this phase.'
);

// FOMO_PIL_LIVE_ENABLED — v0.5.12 Q5.A global kill switch, default false.
expectBoolFalseOrTrue(
  'FOMO_PIL_LIVE_ENABLED',
  'Q5.A global kill switch carry-forward. Default false. v0.5.13 canary FLIPS this to true ONLY during the explicit canary window WHILE FOMO_PIL_LIVE_USER_ALLOWLIST is non-empty.'
);

// FOMO_PIL_DIVERGENCE_AUDIT_ENABLED — stays default off this phase.
expectBoolFalseOrTrue(
  'FOMO_PIL_DIVERGENCE_AUDIT_ENABLED',
  'Hard boundary v0.5.13: FOMO_PIL_DIVERGENCE_AUDIT_ENABLED stays default off this phase. Production divergence audit is its own future 6Q gate.'
);
if ((process.env.FOMO_PIL_DIVERGENCE_AUDIT_ENABLED ?? '').trim() === 'true') {
  issues.push({
    name: 'FOMO_PIL_DIVERGENCE_AUDIT_ENABLED',
    severity: 'error',
    message:
      'FOMO_PIL_DIVERGENCE_AUDIT_ENABLED=true is OUT OF SCOPE for v0.5.13 per founder-locked hard boundary. Future 6Q gate.'
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
/* v0.5.13 — new per-user allowlist (LOAD-BEARING)                        */
/* ---------------------------------------------------------------------- */

const pilLiveEnabledRaw = (process.env.FOMO_PIL_LIVE_ENABLED ?? '').trim();
const pilLiveEnabled = pilLiveEnabledRaw === 'true';
const allowlistRaw = process.env.FOMO_PIL_LIVE_USER_ALLOWLIST;
const allowlistParsed = parsePilLiveAllowlist(allowlistRaw);

// Founder correction #2: ERROR (not warn) when global=true + allowlist empty/unset.
if (pilLiveEnabled && allowlistParsed.length === 0) {
  issues.push({
    name: 'FOMO_PIL_LIVE_USER_ALLOWLIST',
    severity: 'error',
    message:
      'LOAD-BEARING v0.5.13 founder rule: FOMO_PIL_LIVE_ENABLED=true requires FOMO_PIL_LIVE_USER_ALLOWLIST to be non-empty. ' +
      'Empty or unset allowlist when the global kill switch is on means PIL would fail-closed (correct), BUT the canary needs to actually exercise the live path — set the allowlist to the founder user_id (FOMO_FOUNDER_USER_ID value) before booting. ' +
      'Preflight is intentionally LOUD here: ambiguity between "global on but nobody allowed" and operator misconfiguration must be resolved at the boot gate, not at runtime.'
  });
}

// Validate the allowlist shape itself (regardless of global on/off).
if (allowlistRaw !== undefined) {
  const rawEntries = allowlistRaw.split(',');
  const dropped = rawEntries.filter((s) => s.trim().length === 0).length;
  if (dropped > 0 && allowlistRaw.length > 0) {
    issues.push({
      name: 'FOMO_PIL_LIVE_USER_ALLOWLIST',
      severity: 'warn',
      message: `FOMO_PIL_LIVE_USER_ALLOWLIST contains ${dropped} empty entries (trailing comma or double comma). Empty entries will be silently filtered by both preflight and runtime. Clean the value to remove ambiguity.`
    });
  }
  // Detect accidentally lowercased / case-mismatched entries against the
  // declared founder user_id, surfacing as WARN (founder correction #1:
  // exact strings, no normalization).
  const founderId = (process.env.FOMO_FOUNDER_USER_ID ?? '').trim();
  if (founderId.length > 0 && pilLiveEnabled && allowlistParsed.length > 0) {
    const hasFounderExact = allowlistParsed.includes(founderId);
    const hasFounderCaseInsensitive = allowlistParsed.some(
      (s) => s.toLowerCase() === founderId.toLowerCase()
    );
    if (!hasFounderExact && hasFounderCaseInsensitive) {
      issues.push({
        name: 'FOMO_PIL_LIVE_USER_ALLOWLIST',
        severity: 'error',
        message:
          `FOMO_PIL_LIVE_USER_ALLOWLIST contains a case-different match for FOMO_FOUNDER_USER_ID='${founderId}'. ` +
          'Founder rule (correction #1): the parser does NOT lowercase. Strings are compared with === for safety. Fix the allowlist to use the exact founder user_id value.'
      });
    }
    if (!hasFounderExact && !hasFounderCaseInsensitive) {
      issues.push({
        name: 'FOMO_PIL_LIVE_USER_ALLOWLIST',
        severity: 'warn',
        message:
          `FOMO_PIL_LIVE_USER_ALLOWLIST does NOT include FOMO_FOUNDER_USER_ID='${founderId}'. ` +
          'v0.5.13 is the FOUNDER-ONLY canary. If you intended to canary a different user, document it; otherwise add the founder user_id.'
      });
    }
  }
}

/* ---------------------------------------------------------------------- */
/* v0.5.13-specific operator gate                                         */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_V0_5_13_BASELINE_CONFIRMED',
  'true',
  'v0.5.13 baseline gate: founder must run the runbook §1 baseline snapshot BEFORE the canary opens. ' +
    'Snapshot records: current brevio.rank.pil_applied count (expected to be the v0.5.12-merge baseline; future canary-window rows will be the new ones) + sender_importance / sender_suppressed row count + alerts.sender_email_hash NOT NULL count. ' +
    'AFTER capture, set FOMO_V0_5_13_BASELINE_CONFIRMED=true and re-run preflight.'
);

/* ---------------------------------------------------------------------- */
/* PENDING runtime commit warnings                                        */
/* ---------------------------------------------------------------------- */

// KillSwitches.pil_live_user_allowlist field — runtime commit adds it.
let killSwitchAllowlistFieldPresent = false;
try {
  const switches = loadKillSwitches(process.env);
  killSwitchAllowlistFieldPresent =
    'pil_live_user_allowlist' in (switches as unknown as Record<string, unknown>);
} catch {
  killSwitchAllowlistFieldPresent = false;
}
if (!killSwitchAllowlistFieldPresent) {
  issues.push({
    name: 'KILL_SWITCHES_PIL_LIVE_USER_ALLOWLIST',
    severity: 'warn',
    message:
      'KillSwitches.pil_live_user_allowlist PENDING runtime commit. ' +
      'Will add `readonly pil_live_user_allowlist: readonly string[]` to KillSwitches; parsed from FOMO_PIL_LIVE_USER_ALLOWLIST as CSV; each entry .trim()ed; NO lowercasing (founder correction #1); empty entries filtered out. ' +
      'After runtime + smoke PASS this warn disappears.'
  });
}

// Worker-level allowlist gate — runtime commit adds it.
// We can only WARN here because the gate is a call-site check, not an exported symbol.
issues.push({
  name: 'WORKER_ALLOWLIST_GATE_INVARIANT',
  severity: 'warn',
  message:
    'LOCKED INVARIANT (v0.5.13 founder rule): apps/fomo/src/workers/gmail-poll.ts MUST check user_id against killSwitches.pil_live_user_allowlist BEFORE invoking deps.pilLive.buildLivePilContext(...). ' +
    'When user_id is NOT in the allowlist, fall through to the baseline-only path (bit-identical to FOMO_PIL_LIVE_ENABLED=false for that user). ' +
    'When the allowlist is empty AND global=true, fail-closed: ALL users get baseline-only + emit a single boot WARN log fomo.pil_live.allowlist_empty. ' +
    'Add aggregate counter to the cycle audit: messages_pil_skipped_not_in_allowlist. ' +
    'Verification: smoke-evidence C1 (4-case truth table) + canary-window observation + soft rollback rehearsal (C7). ' +
    'After runtime + smoke PASS this reminder disappears.'
});

// Ranker prompt version invariant — both shapes carry-forward from v0.5.12.
const EXPECTED_V0511_RANKER_PROMPT_VERSION = 'ranker-v0.2.0' as string;
const EXPECTED_V0512_RANKER_PROMPT_VERSION = 'ranker-v0.3.0' as string;
const currentRankerPromptVersion = RANKER_PROMPT_VERSION as string;
const currentRankerPromptVersionWithPil = RANKER_PROMPT_VERSION_WITH_PIL as string;
if (currentRankerPromptVersion !== EXPECTED_V0511_RANKER_PROMPT_VERSION) {
  issues.push({
    name: 'RANKER_PROMPT_VERSION',
    severity: 'error',
    message: `Ranker baseline PROMPT_VERSION must remain '${EXPECTED_V0511_RANKER_PROMPT_VERSION}'. Current: '${currentRankerPromptVersion}'.`
  });
}
if (currentRankerPromptVersionWithPil !== EXPECTED_V0512_RANKER_PROMPT_VERSION) {
  issues.push({
    name: 'RANKER_PROMPT_VERSION_WITH_PIL',
    severity: 'error',
    message: `Ranker PIL-block PROMPT_VERSION_WITH_PIL must remain '${EXPECTED_V0512_RANKER_PROMPT_VERSION}'. Current: '${currentRankerPromptVersionWithPil}'.`
  });
}

/* ---------------------------------------------------------------------- */
/* Kill-switch sanity (carry-forward scope-boundary checks)               */
/* ---------------------------------------------------------------------- */

if ((process.env.FOMO_AUTO_SEND_ENABLED ?? '').trim() === 'true') {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message: 'FOMO_AUTO_SEND_ENABLED=true is forbidden in v0.5.13. Auto-send is its own future gate.'
  });
}

if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'error',
    message:
      'BREVIO_DEV_MODE=true skips Postgres persistence. v0.5.13 canary MUST run against real Neon so brevio.rank.pil_applied audits are observable.'
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

console.log(`Parsed FOMO_PIL_LIVE_USER_ALLOWLIST: ${JSON.stringify(allowlistParsed)} (length ${allowlistParsed.length})`);
console.log('');

if (warns.length > 0) {
  console.log('Warnings (PENDING runtime commit / operator reminders):');
  for (const i of warns) {
    console.log(`  [WARN] ${i.name}: ${i.message}`);
  }
  console.log('');
}

console.log('✓ Preflight passed.\n');
console.log('  Next steps (see docs/smoke-test-v0.5.13-pil-live-canary.md):');
console.log('    1. §1 baseline snapshot captured (FOMO_V0_5_13_BASELINE_CONFIRMED=true)');
console.log('    2. Confirm FOMO_PIL_LIVE_USER_ALLOWLIST contains the founder user_id (exact-case match).');
console.log('    3. Boot dev server with kill switch OFF (Phase A bit-identical baseline). Confirm pil_live_enabled=false.');
console.log('    4. Phase A — Kill switch OFF, allowlist any: drive a founder rank, verify ranker-v0.2.0, 0 PIL audits (carry-forward of v0.5.12 C1).');
console.log('    5. Phase B — Kill switch ON, allowlist EMPTY: confirm preflight ERRORS (this script). If runtime is reached, verify fomo.pil_live.allowlist_empty WARN log + 0 PIL audits.');
console.log('    6. Phase C — Kill switch ON, allowlist=founder: drive a founder rank with a matching canonical-HMAC PIL row. Verify ≥1 brevio.rank.pil_applied audit, all 9 fields, ranker-v0.3.0 (C2).');
console.log('    7. Phase D — Cross-user check: verify 0 PIL audits for non-founder user_ids in canary window (C3 LOAD-BEARING).');
console.log('    8. Phase E — Soft rollback rehearsal: clear allowlist mid-canary. Next fresh rank: ranker-v0.2.0, no new PIL audit (C7 LOAD-BEARING).');
console.log('    9. Phase F — Hard rollback rehearsal: flip FOMO_PIL_LIVE_ENABLED=false mid-canary. Next fresh rank: ranker-v0.2.0, no new PIL audit (C8 LOAD-BEARING).');
console.log('   10. Re-run v0.5.12 BB1–BB8 eval — all 3/3 deterministic PASS (C12 regression).');
console.log('   11. Carry-forward: pnpm smoke-evidence:v0.5.9 + smoke-evidence:v0.5.10 + smoke-evidence:v0.5.11 + smoke-evidence:v0.5.12');
console.log('   12. pnpm smoke-evidence:v0.5.13 — expect VERDICT: PASS');
console.log('   13. Fill in docs/SMOKE_REPORT_v0.5.13.md');
console.log('   14. Final state: FOMO_PIL_LIVE_ENABLED=false unless founder explicitly approves keeping ON (founder correction #3).');
