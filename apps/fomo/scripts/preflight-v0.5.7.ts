// Phase v0.5.7 preflight — Human Message Renderer (founder-only smoke).
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-06):
//   This script is part of the v0.5.7 SCAFFOLDING commit, which lands BEFORE
//   the runtime implementation. v0.5.7 introduces ONE new audit kind:
//     - fomo.alert.hmr_degradation_applied
//   AND expects a bump of FOUNDER_TEXT_TEMPLATE_VERSION (currently
//   'founder-text-v0.2.0' — the v0.5.6 deterministic-shell version). The
//   runtime commit bumps it to 'human-message-v0.3.0' to mark the renaming
//   of the renderer layer to surface the Human Message Renderer product
//   principle (see memory feedback_brevio-human-message-renderer-principle).
//
//   These are EXPECTED OUTPUTS of the future runtime commit, not
//   already-existing behaviour. While the runtime commit is pending, the
//   audit kind will be absent from FOMO_AUDIT_ACTIONS and the template
//   version will still be 'founder-text-v0.2.0'. Both are reported as
//   PENDING runtime commit (severity 'warn', exit code 0). Once the
//   runtime commit lands and bumps both, the warns disappear.
//
// Pure config inspection — no DB, no network. Validates that the substrate
// is the v0.5.6-PASS shape (so the founder smoke runs against a known-good
// substrate) AND that the v0.5.7-specific founder gate is set.
//
// v0.5.7 scope (locked Q1–Q6 — see memory project_v05-7-scope):
//   * Q1.A: Two-sentence canonical template: `<Sender> emailed you about
//     <subject_phrase>. <Why_clause>.` Single-sentence-no-subject fallback
//     when subject is empty: `<Sender> emailed you. <Why_clause>.`
//   * Modified Q2.B sender chain (first-name → domain-label → email-local
//     human-readable pattern → "Someone"). NO awkward Galiettemita-style
//     names. NO masked-email in opener.
//   * Q3.B subject naturalization — strip [bracketed] / Re: / Fwd: prefixes
//     only. No noun rewriting. No LLM subject paraphrase.
//   * Q4.A ranker prompt rewrite → `ranker-v0.2.0` (2nd-person, action-
//     oriented rank.reason). Renderer uses verbatim. PRESERVES 3E.1: no
//     new LLM call at body-composition; existing ranker model call gets
//     a new prompt only.
//   * Q5.A locked degradation matrix: each fallback audit-visible via
//     structural-only fields. NO raw subject / body / header in audit detail.
//   * Q6.A with restraint: new core file apps/fomo/src/core/human-message-
//     renderer.ts exporting renderHumanMessage(). Active surface ONLY
//     `email_alert`. `renderFounderText` becomes thin wrapper. Bumped
//     `human-message-v0.3.0`. New audit kind `fomo.alert.hmr_degradation_applied`.
//     New audit fields on fomo.send.attempted detail:
//       - sender_resolution_path
//       - subject_strip_applied
//       - reason_voice
//       - template_shape
//     NOT a multi-surface framework. No surface registry, no plugin system.
//
// Forbidden in v0.5.7 (preserves prior phase locks + permanent product
// principles):
//   * LLM body generation — 3E.1 directive 2026-05-25 PRESERVED. No new
//     drafter LLM at body-composition. The Q4.A ranker prompt rewrite is
//     NOT a new LLM call; it changes the existing ranker prompt.
//   * Personalized Importance Learning substrate — own phase
//   * FOMO_AUTO_SEND_ENABLED=true
//   * BREVIO_DEV_MODE=true
//   * SendBlue tier work — F1 own future phase
//   * Dashboard / web UI
//   * Second HMR surface (calendar / drafts / tasks / etc.) — each own 6Q
//     gate per scope-isolation discipline
//   * SaaS / vendor message renderer — Brevio owns HMR end-to-end
//
// The runbook (docs/smoke-test-v0.5.7-human-message-renderer.md) covers
// out-of-band requirements preflight cannot check:
//   * Baseline snapshot of recent fomo.send.attempted rows captured
//   * Taste-check fixture script run offline (load-bearing per Q5.A C10
//     correction — taste check works WITHOUT SendBlue delivery)
//   * Real iMessage delivery is OPPORTUNISTIC ONLY; N/A if SendBlue
//     OPTED_OUT/tier state still blocks (v0.5.7 is HMR, not SendBlue
//     unblock work)
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
      message: `${name} required for v0.5.7 (${ctx}). Set ≥ ${min} explicitly.`
    });
    return;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || n < min) {
    issues.push({
      name,
      severity: 'error',
      message: `${name}=${raw} is below the v0.5.7 minimum of ${min}. ${ctx}`
    });
  }
}

console.log('Phase v0.5.7 preflight — Human Message Renderer (founder-only smoke)\n');

/* ---------------------------------------------------------------------- */
/* Substrate carry-forward (v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4 + v0.5.5    */
/*                          + v0.5.6)                                      */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required.');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');
requireMin('BREVIO_PHONE_HASH_KEY', 32, 'BREVIO_PHONE_HASH_KEY must be a 32-byte key');

expectEquals(
  'FOMO_FRIEND_BETA_ENABLED',
  'true',
  'v0.5.7 substrate still requires the friend-beta kill switch ON (carry-forward; no friend involved this phase but substrate stays live).'
);

require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required (founder Gmail polling continues).');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required.');
require_(
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE',
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required.'
);

// SendBlue env vars are STILL required because the substrate stays live —
// even if SendBlue blocks delivery, the outbound worker calls SendBlue and
// records the audit. The taste-check fixture (load-bearing per C10) does
// NOT call SendBlue at all (renders bodies offline).
require_('SENDBLUE_API_KEY_ID', 'SENDBLUE_API_KEY_ID required (substrate continues; not load-bearing for C10).');
require_('SENDBLUE_API_SECRET_KEY', 'SENDBLUE_API_SECRET_KEY required.');
require_('SENDBLUE_FROM_NUMBER', 'SENDBLUE_FROM_NUMBER required.');

require_('OPENAI_API_KEY', 'OPENAI_API_KEY required (ranker — Q4.A prompt rewrite is the LLM-allowed slot per 3E.1 carve-out).');
expectEquals('FOMO_RANKER_ENABLED', 'true', 'Ranker must be on — v0.5.7 bumps the prompt to ranker-v0.2.0 (2nd-person, action-oriented).');
expectEquals(
  'FOMO_SLACK_REVIEW_ENABLED',
  'true',
  'Slack review must be on (founder regression flow C11).'
);
expectEquals(
  'FOMO_SEND_ENABLED',
  'true',
  'FOMO_SEND_ENABLED must be true so the outbound worker exercises renderHumanMessage(). Real delivery is opportunistic per C10 correction.'
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
      'FOMO_FOUNDER_PHONE_NUMBER required (substrate continues). Real iMessage delivery is OPPORTUNISTIC per C10 correction — N/A if SendBlue blocks.'
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
      'FOMO_FRIEND_BETA_BASE_URL required (substrate live). HTTPS URL needed for SendBlue inbound webhook even if outbound is blocked.'
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
  'v0.5.7 smoke needs the polling worker live long enough to produce ≥1 synthetic alert with the new HMR template.'
);
checkCycleMin(
  'FOMO_OUTBOUND_MAX_CYCLES',
  30,
  'v0.5.7 smoke needs the outbound worker live long enough to invoke renderHumanMessage on ≥1 approved alert.'
);

/* ---------------------------------------------------------------------- */
/* v0.5.3 + v0.5.5 + v0.5.6 audit registry invariants (carried forward)   */
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
  'fomo.alert.drafter_schema_failed'
] as const satisfies readonly AuditAction[];
// Cast to readonly string[] at SCAFFOLDING time so the Set is Set<string>
// and `.has()` accepts the not-yet-registered EXPECTED_V057_NEW_AUDIT_KIND.
// Runtime commit removes the cast once the kind is in the AuditAction
// union, restoring the strict pattern. Same shape as v0.5.6 scaffolding
// (commit a1159ca3).
const auditActionSet = new Set(FOMO_AUDIT_ACTIONS as readonly string[]);
const missingCarryForward = requiredCarryForwardActions.filter((a) => !auditActionSet.has(a));
if (missingCarryForward.length > 0) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'error',
    message: `Prior-phase audit actions missing from registry (still required for v0.5.7): ${missingCarryForward.join(', ')}`
  });
}

const memorySignalSet = new Set(MEMORY_SIGNAL_KINDS as readonly string[]);
if (!memorySignalSet.has('stop_active')) {
  issues.push({
    name: 'MEMORY_SIGNAL_KINDS',
    severity: 'error',
    message:
      "v0.5.5 invariant carried into v0.5.7: 'stop_active' must stay registered."
  });
}

/* ---------------------------------------------------------------------- */
/* v0.5.7-specific operator gate                                          */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_V0_5_7_BASELINE_CONFIRMED',
  'true',
  'v0.5.7 baseline gate: founder must run the runbook §0 baseline snapshot BEFORE the smoke starts. ' +
    'That snapshot records recent fomo.send.attempted audit rows (template_version + content_chars + the four new HMR audit fields) so criteria C2/C3/C4/C7/C8/C9 can be compared against the v0.5.6 baseline. ' +
    'Set FOMO_V0_5_7_BASELINE_CONFIRMED=true only AFTER you have captured the baseline into /tmp/v0.5.7-baseline-send-attempted.txt.'
);

const windowHours = (process.env.FOMO_V0_5_7_WINDOW_HOURS ?? '').trim();
if (!windowHours) {
  issues.push({
    name: 'FOMO_V0_5_7_WINDOW_HOURS',
    severity: 'warn',
    message:
      'FOMO_V0_5_7_WINDOW_HOURS not set; smoke-evidence will default to 24h. Override only if the smoke runs across sessions.'
  });
} else {
  const n = Number(windowHours);
  if (!Number.isFinite(n) || n < 1 || n > 168) {
    issues.push({
      name: 'FOMO_V0_5_7_WINDOW_HOURS',
      severity: 'error',
      message: `FOMO_V0_5_7_WINDOW_HOURS=${windowHours} outside 1–168.`
    });
  }
}

/* ---------------------------------------------------------------------- */
/* v0.5.7-NEW expected runtime outputs — PENDING runtime commit           */
/* ---------------------------------------------------------------------- */
/*
 * 1. fomo.alert.hmr_degradation_applied audit kind — registered by runtime
 *    commit. Fires when ANY Q5.A degradation fallback rule triggers
 *    (sender_resolution → "Someone", subject empty, reason schema fail,
 *    transitional reason_voice='legacy_3p'). Best-effort, no retry.
 * 2. FOUNDER_TEXT_TEMPLATE_VERSION bump — runtime commit changes from
 *    'founder-text-v0.2.0' to 'human-message-v0.3.0'. The rename surfaces
 *    the HMR product principle in audit (Brevio-owned message renderer,
 *    not a vendor layer). Locked per Q6.A.
 */
// Plain string literal at SCAFFOLDING time — the kind is not yet in the
// AuditAction union (runtime commit widens the union). The runtime commit
// also tightens this constant to `as const satisfies AuditAction` to match
// the carry-forward pattern above. See same-shape comment in
// scripts/preflight-v0.5.6.ts at scaffolding time (commit a1159ca3).
const EXPECTED_V057_NEW_AUDIT_KIND = 'fomo.alert.hmr_degradation_applied';
if (!auditActionSet.has(EXPECTED_V057_NEW_AUDIT_KIND)) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'warn',
    message: `v0.5.7 expected audit kind PENDING runtime commit: '${EXPECTED_V057_NEW_AUDIT_KIND}'. This is normal at scaffolding time — the kind is registered by the future runtime implementation commit when wiring the Q5.A degradation matrix.`
  });
}

// Typed as `string` (not the literal) so the runtime check stays meaningful
// after the runtime commit bumped FOUNDER_TEXT_TEMPLATE_VERSION from this
// baseline. Without the widening, tsc would correctly flag the equality as
// statically impossible.
const V056_TEMPLATE_VERSION_BASELINE: string = 'founder-text-v0.2.0';
if (FOUNDER_TEXT_TEMPLATE_VERSION === V056_TEMPLATE_VERSION_BASELINE) {
  issues.push({
    name: 'FOUNDER_TEXT_TEMPLATE_VERSION',
    severity: 'warn',
    message: `v0.5.7 expected template-version bump PENDING runtime commit: FOUNDER_TEXT_TEMPLATE_VERSION is still '${V056_TEMPLATE_VERSION_BASELINE}'. The runtime commit bumps this to 'human-message-v0.3.0' to mark the HMR rename (see memory feedback_brevio-human-message-renderer-principle). Once bumped, this warn disappears and smoke-evidence can prove the new shape is in effect by querying fomo.send.attempted detail.template_version.`
  });
}

/* ---------------------------------------------------------------------- */
/* 3E.1 directive guardrail — code-level                                  */
/* ---------------------------------------------------------------------- */
/*
 * The 3E.1 directive (2026-05-25) bans LLM body generation. v0.5.7 PRESERVES
 * this by design: the Q4.A ranker prompt rewrite changes the EXISTING ranker
 * model call's prompt; it does NOT add a new LLM call at body-composition
 * time. The new renderHumanMessage function is PURE — no I/O, no LLM client.
 *
 * There is no automated check here at preflight time. This comment exists as
 * a tripwire: if any future preflight is tempted to add a check like
 *   require_('FOMO_HMR_LLM_MODEL_ID', ...)
 * that would be evidence of reversing 3E.1 — STOP and confirm with founder
 * + update memory feedback_3e1-no-llm-body-generation.md.
 */

/* ---------------------------------------------------------------------- */
/* Forbidden in v0.5.7                                                    */
/* ---------------------------------------------------------------------- */

const switches = loadKillSwitches(process.env);
if (switches.auto_send_enabled) {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message:
      'v0.5.7 hard boundary: founder Slack review still required for FOMO alerts. Set FOMO_AUTO_SEND_ENABLED=false (or unset). Auto-send is its own future 6Q gate per FOMO_PLAN v0.8.'
  });
}
if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'error',
    message:
      'BREVIO_DEV_MODE=true is a hard error in v0.5.7 — ephemeral per-process keys would invalidate the founder OAuth tokens between restarts.'
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
  console.log('  Next steps (see docs/smoke-test-v0.5.7-human-message-renderer.md):');
  console.log('    1. §0 baseline snapshot captured (FOMO_V0_5_7_BASELINE_CONFIRMED=true)');
  console.log('    2. Boot dev server: pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.7.log');
  console.log('    3. Run §6 Test 1 (mock regression: synthetic important email → renderHumanMessage → assert template_version human-message-v0.3.0, four new audit fields populated, no email-content leak)');
  console.log('    4. Run §6 Test 2 (Q5.A degradation matrix: induce empty subject + empty sender_name + reason schema fail; assert fomo.alert.hmr_degradation_applied fires per fallback path)');
  console.log('    5. Run §6 Test 3 (load-bearing taste check: render N representative bodies via taste-check fixture script; founder eye-tests against the Q1.A natural-sentence bar. If SendBlue allows delivery, ALSO eye-test on iPhone.)');
  console.log('    6. Run §6 Test 4 (cross-tenant — other users untouched)');
  console.log('    7. Run all 7 evidence scripts: pnpm smoke-evidence:v0.5.1 && pnpm smoke-evidence:v0.5.2 && pnpm smoke-evidence:v0.5.3 && pnpm smoke-evidence:v0.5.4 && pnpm smoke-evidence:v0.5.5 && pnpm smoke-evidence:v0.5.6 && pnpm smoke-evidence:v0.5.7');
  console.log('    8. Fill in docs/SMOKE_REPORT_v0.5.7.md');
  console.log('');
  const pendingCount =
    (auditActionSet.has(EXPECTED_V057_NEW_AUDIT_KIND) ? 0 : 1) +
    (FOUNDER_TEXT_TEMPLATE_VERSION === V056_TEMPLATE_VERSION_BASELINE ? 1 : 0);
  if (pendingCount > 0) {
    console.log(
      `  NOTE: ${pendingCount} v0.5.7 runtime artifact(s) are PENDING runtime commit. Smoke-evidence will report some criteria as PENDING until the runtime implementation lands. This is expected at scaffolding time.`
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
