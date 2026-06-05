// Phase v0.5.6 preflight — iMessage Tone + Summary Length (founder-only smoke).
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-05, Q3 corrected after
// mid-scaffolding discovery of the 3E.1 no-LLM-body-generation directive):
//   This script is part of the v0.5.6 SCAFFOLDING commit, which lands BEFORE
//   the runtime implementation. v0.5.6 introduces ONE new audit kind:
//     - fomo.alert.drafter_schema_failed
//   AND expects a bump of FOUNDER_TEXT_TEMPLATE_VERSION (currently
//   'founder-text-v0.1.0' — runtime commit bumps it to mark the shape
//   change so the audit trail can diff old-vs-new template renderings).
//
//   These are EXPECTED OUTPUTS of the future runtime commit, not
//   already-existing behaviour. While the runtime commit is pending, the
//   audit kind will be absent from FOMO_AUDIT_ACTIONS and the template
//   version will still be 'founder-text-v0.1.0'. Both are reported as
//   PENDING runtime commit (severity 'warn', exit code 0). Once the
//   runtime commit lands and bumps both, the warns disappear.
//
// Pure config inspection — no DB, no network. Validates that the substrate
// is the v0.5.5-PASS shape (so the founder smoke runs against a known-good
// substrate) AND that the v0.5.6-specific founder gate is set.
//
// v0.5.6 HYBRID scope (Q3 corrected lock):
//   * Surface (a): deterministic shell rewrite in apps/fomo/src/core/founder-
//     text-template.ts — drops "FOMO · IMPORTANT (0.92)" telemetry header,
//     sentence-shaped composition from safe fields, sentence-boundary
//     truncation (NO arbitrary ellipsis per founder Q4 correction).
//   * Surface (b): ranker `reason` field tightening (apps/fomo/src/ranker/
//     prompt.ts + openai-response-format.ts + validator.ts) — the ONLY
//     LLM-allowed slot per 3E.1 carve-out. Prompt rewrite for voice;
//     structured output schema with typed length on `reason`; deterministic
//     fallback when ranker.reason violates schema.
//
// Forbidden in v0.5.6 (preserves prior phase locks + 3E.1):
//   * LLM body generation — 3E.1 directive 2026-05-25 PRESERVED. No new
//     drafter LLM at body-generation step.
//   * FOMO_AUTO_SEND_ENABLED=true
//   * BREVIO_DEV_MODE=true
//   * Changes to v0.5.5 STOP enforcement path
//   * Per-user tone customization (PIL-adjacent, future)
//
// The runbook (docs/smoke-test-v0.5.6-imessage-tone.md) covers out-of-band
// requirements preflight cannot check:
//   * Baseline snapshot of recent fomo.send.attempted rows captured
//   * Founder un-flagged from SendBlue OPTED_OUT (one-time ops; required
//     for the manual real-iMessage taste check)
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
      message: `${name} required for v0.5.6 (${ctx}). Set ≥ ${min} explicitly.`
    });
    return;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || n < min) {
    issues.push({
      name,
      severity: 'error',
      message: `${name}=${raw} is below the v0.5.6 minimum of ${min}. ${ctx}`
    });
  }
}

console.log('Phase v0.5.6 preflight — iMessage Tone + Summary Length (founder-only smoke)\n');

/* ---------------------------------------------------------------------- */
/* Substrate carry-forward (v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4 + v0.5.5)   */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required.');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');
requireMin('BREVIO_PHONE_HASH_KEY', 32, 'BREVIO_PHONE_HASH_KEY must be a 32-byte key');

expectEquals(
  'FOMO_FRIEND_BETA_ENABLED',
  'true',
  'v0.5.6 substrate still requires the friend-beta kill switch ON (carry-forward; no friend is involved this phase but the substrate stays live).'
);

require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required (founder Gmail polling continues).');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required.');
require_(
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE',
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required.'
);

require_('SENDBLUE_API_KEY_ID', 'SENDBLUE_API_KEY_ID required (real-iMessage taste check).');
require_('SENDBLUE_API_SECRET_KEY', 'SENDBLUE_API_SECRET_KEY required.');
require_('SENDBLUE_FROM_NUMBER', 'SENDBLUE_FROM_NUMBER required.');

require_('OPENAI_API_KEY', 'OPENAI_API_KEY required (ranker — reason field is the LLM-allowed slot per 3E.1 carve-out).');
expectEquals('FOMO_RANKER_ENABLED', 'true', 'Ranker must be on — v0.5.6 tightens the reason field prompt + schema.');
expectEquals(
  'FOMO_SLACK_REVIEW_ENABLED',
  'true',
  'Slack review must be on (founder regression flow C11).'
);
expectEquals(
  'FOMO_SEND_ENABLED',
  'true',
  'FOMO_SEND_ENABLED must be true so the manual real-iMessage taste check can fire.'
);
expectEquals(
  'FOMO_GMAIL_POLLING_ENABLED',
  'true',
  'FOMO_GMAIL_POLLING_ENABLED must be true so a synthetic important email can produce a real alert.'
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
      'FOMO_FOUNDER_PHONE_NUMBER required for the manual real-iMessage taste check. This IS the device that receives the v0.5.6 alert preview.'
  });
} else if (/^\+1555010\d{4}$/.test(founderPhone)) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message: `FOMO_FOUNDER_PHONE_NUMBER='+1555010xxxx' is NANPA-reserved fictional. v0.5.6 manual taste check requires a real founder phone.`
  });
}

const friendBaseUrl = (process.env.FOMO_FRIEND_BETA_BASE_URL ?? '').trim();
if (!friendBaseUrl) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_BASE_URL',
    severity: 'error',
    message:
      'FOMO_FRIEND_BETA_BASE_URL required. SendBlue delivery (manual taste check) needs the public HTTPS URL alive.'
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
  'v0.5.6 mock-SendBlue smoke needs the polling worker live long enough to produce ≥1 synthetic alert with the new template.'
);
checkCycleMin(
  'FOMO_OUTBOUND_MAX_CYCLES',
  30,
  'v0.5.6 mock-SendBlue smoke needs the outbound worker live long enough to invoke renderFounderText on ≥1 approved alert.'
);

/* ---------------------------------------------------------------------- */
/* v0.5.3 + v0.5.5 audit registry invariants (carried forward)            */
/* ---------------------------------------------------------------------- */

// Strictly typed via `as const satisfies readonly AuditAction[]`. Mirrors
// the v0.5.5 founder directive ("After runtime lands, remove/avoid any
// loose string-cast workaround that hides missing audit action
// registration"): if anyone removes one of these from FOMO_AUDIT_ACTIONS
// in a future PR, tsc fails here.
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
  'fomo.poll.skipped_stop_active'
] as const satisfies readonly AuditAction[];
const auditActionSet = new Set(FOMO_AUDIT_ACTIONS);
const missingCarryForward = requiredCarryForwardActions.filter((a) => !auditActionSet.has(a));
if (missingCarryForward.length > 0) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'error',
    message: `Prior-phase audit actions missing from registry (still required for v0.5.6): ${missingCarryForward.join(', ')}`
  });
}

const memorySignalSet = new Set(MEMORY_SIGNAL_KINDS as readonly string[]);
if (!memorySignalSet.has('stop_active')) {
  issues.push({
    name: 'MEMORY_SIGNAL_KINDS',
    severity: 'error',
    message:
      "v0.5.5 invariant carried into v0.5.6: 'stop_active' must stay registered."
  });
}

/* ---------------------------------------------------------------------- */
/* v0.5.6-specific operator gate                                          */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_V0_5_6_BASELINE_CONFIRMED',
  'true',
  'v0.5.6 baseline gate: founder must run the runbook §0 baseline snapshot BEFORE the smoke starts. ' +
    'That snapshot records recent fomo.send.attempted audit rows (template_version + content_chars) so criteria C2/C3/C4 can be compared against the v0.5.5 baseline. ' +
    'Set FOMO_V0_5_6_BASELINE_CONFIRMED=true only AFTER you have captured the baseline into /tmp/v0.5.6-baseline-send-attempted.txt.'
);

const windowHours = (process.env.FOMO_V0_5_6_WINDOW_HOURS ?? '').trim();
if (!windowHours) {
  issues.push({
    name: 'FOMO_V0_5_6_WINDOW_HOURS',
    severity: 'warn',
    message:
      'FOMO_V0_5_6_WINDOW_HOURS not set; smoke-evidence will default to 24h. Override only if the smoke runs across sessions.'
  });
} else {
  const n = Number(windowHours);
  if (!Number.isFinite(n) || n < 1 || n > 168) {
    issues.push({
      name: 'FOMO_V0_5_6_WINDOW_HOURS',
      severity: 'error',
      message: `FOMO_V0_5_6_WINDOW_HOURS=${windowHours} outside 1–168.`
    });
  }
}

/* ---------------------------------------------------------------------- */
/* v0.5.6-NEW expected runtime outputs — PENDING runtime commit           */
/* ---------------------------------------------------------------------- */
/*
 * 1. fomo.alert.drafter_schema_failed audit kind — registered by runtime
 *    commit when ranker.reason fails the new structured-output schema and
 *    the deterministic fallback string is substituted.
 * 2. FOUNDER_TEXT_TEMPLATE_VERSION bump — runtime commit changes from
 *    'founder-text-v0.1.0' to e.g. 'founder-text-v0.2.0' to mark the
 *    deterministic-shell rewrite (drops "FOMO · IMPORTANT (0.92)" header,
 *    sentence-shaped composition, sentence-boundary truncation, ranker.reason
 *    substituted in place of body_snippet).
 */
// Same strict-typing pattern as the carry-forward list above. Runtime
// commit registers this kind, so `as const satisfies` narrows to the
// AuditAction union at compile time.
const EXPECTED_V056_NEW_AUDIT_KIND = 'fomo.alert.drafter_schema_failed' as const satisfies AuditAction;
if (!auditActionSet.has(EXPECTED_V056_NEW_AUDIT_KIND)) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'warn',
    message: `v0.5.6 expected audit kind PENDING runtime commit: '${EXPECTED_V056_NEW_AUDIT_KIND}'. This is normal at scaffolding time — the kind is registered by the future runtime implementation commit.`
  });
}

// Typed as `string` (not the literal) so the runtime check stays
// meaningful after the runtime commit bumped FOUNDER_TEXT_TEMPLATE_VERSION
// from this baseline. Without the widening, tsc would correctly flag the
// equality as statically impossible.
const V055_TEMPLATE_VERSION_BASELINE: string = 'founder-text-v0.1.0';
if (FOUNDER_TEXT_TEMPLATE_VERSION === V055_TEMPLATE_VERSION_BASELINE) {
  issues.push({
    name: 'FOUNDER_TEXT_TEMPLATE_VERSION',
    severity: 'warn',
    message: `v0.5.6 expected template-version bump PENDING runtime commit: FOUNDER_TEXT_TEMPLATE_VERSION is still '${V055_TEMPLATE_VERSION_BASELINE}'. The runtime commit bumps this to mark the deterministic-shell rewrite (e.g. 'founder-text-v0.2.0'). Once bumped, this warn disappears and smoke-evidence can prove the new shape is in effect by querying fomo.send.attempted detail.template_version.`
  });
}

/* ---------------------------------------------------------------------- */
/* 3E.1 directive guardrail — code-level                                  */
/* ---------------------------------------------------------------------- */
/*
 * The 3E.1 directive (2026-05-25) bans LLM body generation. v0.5.6 PRESERVES
 * this. There is no automated check here at preflight time — the directive
 * lives at the architectural level (do not introduce a new LLM call in
 * renderFounderText or its callers) and is checked by code review + the
 * v0.5.6 PR boundary. This comment exists as a tripwire: if any future
 * preflight is tempted to add a check like
 *   require_('FOMO_DRAFTER_LLM_MODEL_ID', ...)
 * that would be evidence of reversing 3E.1 — STOP and confirm with founder
 * + update memory feedback_3e1-no-llm-body-generation.md.
 */

/* ---------------------------------------------------------------------- */
/* Forbidden in v0.5.6                                                    */
/* ---------------------------------------------------------------------- */

const switches = loadKillSwitches(process.env);
if (switches.auto_send_enabled) {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message:
      'v0.5.6 hard boundary: founder Slack review still required for FOMO alerts. Set FOMO_AUTO_SEND_ENABLED=false (or unset).'
  });
}
if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'error',
    message:
      'BREVIO_DEV_MODE=true is a hard error in v0.5.6 — ephemeral per-process keys would invalidate the founder OAuth tokens between restarts.'
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
  console.log('  Next steps (see docs/smoke-test-v0.5.6-imessage-tone.md):');
  console.log('    1. §0 baseline snapshot captured (FOMO_V0_5_6_BASELINE_CONFIRMED=true)');
  console.log('    2. Boot dev server: pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.6.log');
  console.log('    3. Run §6 Test 1 (mock-SendBlue regression: synthetic important email → ranker.reason → renderFounderText → assert template_version v0.2.0, content_chars 220–320, no ellipsis, no "FOMO ·" header)');
  console.log('    4. Run §6 Test 2 (schema-violation fallback: stub ranker to return >180-char reason; assert fomo.alert.drafter_schema_failed audit + deterministic fallback substituted)');
  console.log('    5. Run §6 Test 3 (manual real-iMessage taste check: founder un-flags OPTED_OUT, sends one synthetic alert to themselves, evaluates rendering on iPhone)');
  console.log('    6. Run §6 Test 4 (cross-tenant — other users untouched)');
  console.log('    7. Run all 6 evidence scripts: pnpm smoke-evidence:v0.5.1 && pnpm smoke-evidence:v0.5.2 && pnpm smoke-evidence:v0.5.3 && pnpm smoke-evidence:v0.5.4 && pnpm smoke-evidence:v0.5.5 && pnpm smoke-evidence:v0.5.6');
  console.log('    8. Fill in docs/SMOKE_REPORT_v0.5.6.md');
  console.log('');
  const pendingCount =
    (auditActionSet.has(EXPECTED_V056_NEW_AUDIT_KIND) ? 0 : 1) +
    (FOUNDER_TEXT_TEMPLATE_VERSION === V055_TEMPLATE_VERSION_BASELINE ? 1 : 0);
  if (pendingCount > 0) {
    console.log(
      `  NOTE: ${pendingCount} v0.5.6 runtime artifact(s) are PENDING runtime commit. Smoke-evidence will report some criteria as PENDING until the runtime implementation lands. This is expected at scaffolding time.`
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
