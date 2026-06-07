// Phase v0.5.13 smoke-evidence — Founder-only PIL Live Canary / Controlled Activation.
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-07):
//   Same 'pending' severity model as v0.5.5–v0.5.12. PENDING means "this
//   criterion depends on a runtime artifact that the runtime commit will
//   introduce." Until the runtime commit lands:
//     * KillSwitches.pil_live_user_allowlist absent → C1 PENDING
//     * Worker-level allowlist gate absent → C2/C3/C6/C7 PENDING
//     * Canary not yet run → C2/C3/C4/C5/C6/C7/C8 PENDING
//   When runtime + canary run, PENDINGs flip to PASS/FAIL per live state.
//
// v0.5.13 scope (locked — see memory project_v05-13-scope):
//   * Add FOMO_PIL_LIVE_USER_ALLOWLIST env var (comma-separated user_id list).
//   * Founder-only canary; per-user allowlist gate in gmail-poll worker.
//   * No new audit kinds; reuses v0.5.12 brevio.rank.pil_applied.
//   * No new memory_signal kinds; reuses v0.5.11/v0.5.12 substrate.
//   * Final production state: FOMO_PIL_LIVE_ENABLED=false unless founder
//     explicitly approves keeping ON after canary review.
//
// HARD INVARIANTS (each enforced by ≥1 criterion):
//   - 4-case truth table for (global, allowlist) (C1) — unit-test confirmed
//   - ≥1 founder PIL audit in canary window (C2)
//   - 0 non-founder PIL audits in canary window (C3 LOAD-BEARING)
//   - pil_score_delta distribution + cap-bind rate < 25% (C4)
//   - Privacy canary 0 hits on PIL audit detail (C5)
//   - Cross-user contamination: 0 founder audits cite non-founder scope (C6 LOAD-BEARING)
//   - Soft rollback (clear allowlist mid-canary) → next rank ranker-v0.2.0 (C7 LOAD-BEARING)
//   - Hard rollback (global=false mid-canary) → next rank ranker-v0.2.0 (C8 LOAD-BEARING)
//   - Legacy message:* rows still ignored at read side (C9)
//   - v0.5.11 substrate UNCHANGED (C10)
//   - v0.5.9/v0.5.10/v0.5.12 carry-forward (C11)
//   - v0.5.12 BB1-BB8 regression (C12)
//
// Founder-only smoke. No friend involvement (three-friend cap). Read-only —
// never mutates the DB.

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import { FOMO_AUDIT_ACTIONS } from '../src/core/audit.js';
import { MEMORY_SIGNAL_KINDS } from '../src/memory/memory-signals.js';
import { loadKillSwitches } from '../src/core/kill-switches.js';
import { PROMPT_VERSION as REPLY_PARSER_PROMPT_VERSION } from '../src/reply-parser/prompt.js';
import { PROMPT_VERSION as RANKER_PROMPT_VERSION } from '../src/ranker/prompt.js';
import { FOUNDER_TEXT_TEMPLATE_VERSION } from '../src/core/founder-text-template.js';

type Severity = 'pass' | 'warn' | 'fail' | 'pending';

interface Finding {
  readonly severity: Severity;
  readonly criterion: string;
  readonly detail: string;
}

const SMOKE_WINDOW_HOURS = Number((process.env.FOMO_V0_5_13_WINDOW_HOURS ?? '24').trim()) || 24;
const FOUNDER_USER_ID = (process.env.FOMO_FOUNDER_USER_ID ?? 'founder').trim();

// Operator-provided rollback timestamps (optional; set by the runbook when
// the canary actually flips the env var). When unset, C7 + C8 fall through
// to PENDING — rollback rehearsal hasn't been observed yet.
const CANARY_OPEN_TS = (process.env.FOMO_V0_5_13_CANARY_OPEN_TS ?? '').trim();
const SOFT_ROLLBACK_TS = (process.env.FOMO_V0_5_13_SOFT_ROLLBACK_TS ?? '').trim();
const HARD_ROLLBACK_TS = (process.env.FOMO_V0_5_13_HARD_ROLLBACK_TS ?? '').trim();

const EXPECTED_V0510_PROMPT_VERSION = 'reply-parser-v0.2.0' as string;
const EXPECTED_V0511_RANKER_BASELINE = 'ranker-v0.2.0' as string;
const EXPECTED_V0512_RANKER_PIL = 'ranker-v0.3.0' as string;
const EXPECTED_HMR_TEMPLATE_VERSION = 'human-message-v0.3.0' as string;

// v0.5.12 PIL audit kind (carry-forward)
const PIL_APPLIED_KIND = 'brevio.rank.pil_applied';

// v0.5.13 cap-bind rate threshold: cap binding > 25% in the canary window
// is a smell (model leaning hard on the prior). Surface as WARN, not FAIL —
// founder review decides if it's a real issue.
const CAP_BIND_RATE_WARN_THRESHOLD = 0.25;

// Privacy canary — same shape as v0.5.12 + canary-specific additions.
const FORBIDDEN_DETAIL_SUBSTRINGS = [
  'Subject:',
  'From:',
  'To:',
  '@gmail.com',
  '@icloud.com',
  '@hotmail.com',
  '@yahoo.com',
  'ignore this',
  'not important',
  'this mattered',
  'more like this',
  'noreply@',
  'unsubscribe',
  'because the user',
  'the user has marked',
  'user previously ignored'
] as const;

function symbol(s: Severity): string {
  switch (s) {
    case 'pass':
      return '✓';
    case 'warn':
      return '!';
    case 'pending':
      return '…';
    case 'fail':
      return '✗';
  }
}

function quantile(sorted: readonly number[], q: number): number {
  if (sorted.length === 0) return NaN;
  const idx = Math.min(sorted.length - 1, Math.floor(q * sorted.length));
  return sorted[idx]!;
}

async function main(): Promise<void> {
  if (!(process.env.DATABASE_URL ?? '').trim()) {
    process.stderr.write('[smoke-evidence:v0.5.13] DATABASE_URL not set. Source .env first.\n');
    process.exit(2);
  }
  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    process.stderr.write(`[smoke-evidence:v0.5.13] DB unavailable: ${dbResult.reason}\n`);
    process.exit(1);
  }
  const db = dbResult.client;

  console.log('Phase v0.5.13 evidence — Founder-only PIL Live Canary / Controlled Activation\n');
  console.log(`Smoke window: last ${SMOKE_WINDOW_HOURS}h (override via FOMO_V0_5_13_WINDOW_HOURS).`);
  console.log(`Founder user_id: ${FOUNDER_USER_ID}`);
  console.log(`Canary open ts: ${CANARY_OPEN_TS || '<unset — set FOMO_V0_5_13_CANARY_OPEN_TS to scope the canary window>'}`);
  console.log(`Soft rollback ts: ${SOFT_ROLLBACK_TS || '<unset — set after the rehearsal>'}`);
  console.log(`Hard rollback ts: ${HARD_ROLLBACK_TS || '<unset — set after the rehearsal>'}`);
  console.log('');

  const findings: Finding[] = [];

  /* ============================================================== */
  /* Registry + runtime-artifact probes — drive PENDING flags        */
  /* ============================================================== */

  const auditActionSet = new Set<string>(FOMO_AUDIT_ACTIONS as readonly string[]);
  const memorySignalSet = new Set<string>(MEMORY_SIGNAL_KINDS as readonly string[]);

  const pilAppliedRegistered = auditActionSet.has(PIL_APPLIED_KIND);
  const signalAggregatedRegistered = auditActionSet.has('brevio.signal.aggregated');
  const senderImportanceRegistered = memorySignalSet.has('sender_importance');
  const senderSuppressedRegistered = memorySignalSet.has('sender_suppressed');
  const pilKindsRegistered = senderImportanceRegistered && senderSuppressedRegistered;

  // v0.5.12 buildLivePilContext probe (carry-forward; v0.5.13 reuses it).
  let livePilContextModulePresent = false;
  try {
    const modulePath = '../src/ranker/pil-context.js';
    const mod = (await import(modulePath)) as Record<string, unknown>;
    livePilContextModulePresent = typeof mod.buildLivePilContext === 'function';
  } catch {
    livePilContextModulePresent = false;
  }

  // v0.5.13-new: KillSwitches.pil_live_user_allowlist field probe.
  let killSwitchAllowlistFieldPresent = false;
  try {
    const switches = loadKillSwitches(process.env);
    killSwitchAllowlistFieldPresent =
      'pil_live_user_allowlist' in (switches as unknown as Record<string, unknown>);
  } catch {
    killSwitchAllowlistFieldPresent = false;
  }

  // alerts.sender_email_hash column (v0.5.11 carry-forward, still load-bearing).
  let alertsHashColumnPresent = false;
  try {
    const colCheck = await db.execute<{ column_name: string }>(
      sql`SELECT column_name FROM information_schema.columns
          WHERE table_name = 'alerts' AND column_name = 'sender_email_hash'`
    );
    alertsHashColumnPresent = (colCheck.rows as Array<{ column_name: string }>).length > 0;
  } catch {
    alertsHashColumnPresent = false;
  }

  const runtimePending = !killSwitchAllowlistFieldPresent;

  // v0.5.11 substrate carry-forward — load-bearing for C10
  const v0511SubstrateIntact =
    pilKindsRegistered &&
    signalAggregatedRegistered &&
    alertsHashColumnPresent;

  // v0.5.7 + v0.5.10 + v0.5.12 carry-forward
  const hmrUnchanged = FOUNDER_TEXT_TEMPLATE_VERSION === EXPECTED_HMR_TEMPLATE_VERSION;
  const replyParserUnchanged = (REPLY_PARSER_PROMPT_VERSION as string) === EXPECTED_V0510_PROMPT_VERSION;
  const rankerPromptVersion = RANKER_PROMPT_VERSION as string;
  const rankerPromptVersionValid =
    rankerPromptVersion === EXPECTED_V0511_RANKER_BASELINE ||
    rankerPromptVersion === EXPECTED_V0512_RANKER_PIL;

  /* ------------------------------------------------------------------ */
  /* C1: 4-case truth table for (global, allowlist)                     */
  /* ------------------------------------------------------------------ */

  if (!killSwitchAllowlistFieldPresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C1: 4-case truth table for (FOMO_PIL_LIVE_ENABLED, FOMO_PIL_LIVE_USER_ALLOWLIST)',
      detail:
        `KillSwitches.pil_live_user_allowlist field PENDING runtime commit. ` +
        `Operator confirms via unit-test PASS on the 4 cases: ` +
        `(a) global=false + any list → all users baseline; ` +
        `(b) global=true + empty list → preflight ERRORS, runtime fail-closed; ` +
        `(c) global=true + list=[founder] → founder hybrid, others baseline; ` +
        `(d) global=true + list=[founder,X] → both hybrid (contract; not exercised in this phase).`
    });
  } else {
    const switches = loadKillSwitches(process.env);
    const allowlist =
      (switches as unknown as { pil_live_user_allowlist?: readonly string[] }).pil_live_user_allowlist ?? [];
    const globalOn = switches.pil_live_enabled === true;
    findings.push({
      severity: 'pending',
      criterion: 'C1: 4-case truth table for (FOMO_PIL_LIVE_ENABLED, FOMO_PIL_LIVE_USER_ALLOWLIST)',
      detail:
        `Boot state: global=${globalOn} allowlist=${JSON.stringify(allowlist)}. ` +
        `OPERATOR-CONFIRMED via unit tests (kill-switches.test.ts + gmail-poll.allowlist.test.ts must include all 4 cases). ` +
        `Cases B/C/D additionally exercised in vivo during the canary window.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C2: ≥1 founder brevio.rank.pil_applied audit in canary window      */
  /* ------------------------------------------------------------------ */

  if (!pilAppliedRegistered || !livePilContextModulePresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C2: ≥1 founder brevio.rank.pil_applied audit in canary window',
      detail:
        `v0.5.12 substrate not present (pilAppliedRegistered=${pilAppliedRegistered}, ` +
        `livePilContextModule=${livePilContextModulePresent}). v0.5.13 requires v0.5.12 on main.`
    });
  } else if (!CANARY_OPEN_TS) {
    findings.push({
      severity: 'pending',
      criterion: 'C2: ≥1 founder brevio.rank.pil_applied audit in canary window',
      detail:
        `FOMO_V0_5_13_CANARY_OPEN_TS unset. Set this env var to the canary-open timestamp ` +
        `(when global=true + allowlist=[founder] was first applied) so this check scopes to the canary window.`
    });
  } else {
    const founderAuditsInWindow = await db.execute<{ n: string }>(
      sql`SELECT COUNT(*)::text AS n FROM audit_log
          WHERE action = ${PIL_APPLIED_KIND}
            AND actor_user_id = ${FOUNDER_USER_ID}
            AND occurred_at >= ${CANARY_OPEN_TS}::timestamptz`
    );
    const n = Number((founderAuditsInWindow.rows[0] as { n: string } | undefined)?.n ?? '0');
    if (n >= 1) {
      findings.push({
        severity: 'pass',
        criterion: 'C2: ≥1 founder brevio.rank.pil_applied audit in canary window',
        detail: `${n} founder PIL audit(s) since ${CANARY_OPEN_TS}. Canary exercised the live path.`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C2: ≥1 founder brevio.rank.pil_applied audit in canary window',
        detail:
          `0 founder PIL audits since ${CANARY_OPEN_TS}. Canary did not exercise the live path — either no founder ranks ` +
          `matched a canonical-HMAC PIL row, OR the allowlist gate is misconfigured, OR runtime regression. ` +
          `Substitute via runbook §7.2 sanctioned synthetic-seed (extend _smoke-v0.5.12-seed-path-c.ts).`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C3 LOAD-BEARING: 0 non-founder PIL audits in canary window         */
  /* ------------------------------------------------------------------ */

  if (!pilAppliedRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C3 (LOAD-BEARING): 0 non-founder brevio.rank.pil_applied audits in canary window',
      detail: 'brevio.rank.pil_applied audit kind not yet registered (v0.5.12 prerequisite).'
    });
  } else if (!CANARY_OPEN_TS) {
    findings.push({
      severity: 'pending',
      criterion: 'C3 (LOAD-BEARING): 0 non-founder brevio.rank.pil_applied audits in canary window',
      detail: 'Set FOMO_V0_5_13_CANARY_OPEN_TS to scope the check.'
    });
  } else {
    const nonFounderAudits = await db.execute<{ n: string }>(
      sql`SELECT COUNT(*)::text AS n FROM audit_log
          WHERE action = ${PIL_APPLIED_KIND}
            AND actor_user_id <> ${FOUNDER_USER_ID}
            AND occurred_at >= ${CANARY_OPEN_TS}::timestamptz`
    );
    const n = Number((nonFounderAudits.rows[0] as { n: string } | undefined)?.n ?? '0');
    if (n === 0) {
      findings.push({
        severity: 'pass',
        criterion: 'C3 (LOAD-BEARING): 0 non-founder brevio.rank.pil_applied audits in canary window',
        detail: `0 non-founder PIL audits since ${CANARY_OPEN_TS}. Per-user allowlist gate working.`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C3 (LOAD-BEARING): 0 non-founder brevio.rank.pil_applied audits in canary window',
        detail:
          `${n} non-founder PIL audit(s) found in canary window — ALLOWLIST GATE LEAK. ` +
          `Inspect: SELECT actor_user_id, occurred_at, detail->>'scope_key_hash' FROM audit_log WHERE action='${PIL_APPLIED_KIND}' AND actor_user_id <> '${FOUNDER_USER_ID}' AND occurred_at >= '${CANARY_OPEN_TS}'.`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C4: pil_score_delta distribution + cap-bind rate < 25%             */
  /* ------------------------------------------------------------------ */

  if (!pilAppliedRegistered || !CANARY_OPEN_TS) {
    findings.push({
      severity: 'pending',
      criterion: 'C4: pil_score_delta distribution + cap-bind rate < 25%',
      detail: 'Pending v0.5.12 substrate + canary open timestamp.'
    });
  } else {
    const audits = await db.execute<{
      pil_score_delta: number;
      pil_score_delta_was_capped: boolean;
    }>(
      sql`SELECT
            (detail->>'pil_score_delta')::float AS pil_score_delta,
            (detail->>'pil_score_delta_was_capped')::boolean AS pil_score_delta_was_capped
          FROM audit_log
          WHERE action = ${PIL_APPLIED_KIND}
            AND actor_user_id = ${FOUNDER_USER_ID}
            AND occurred_at >= ${CANARY_OPEN_TS}::timestamptz`
    );
    const rows = audits.rows as Array<{
      pil_score_delta: number;
      pil_score_delta_was_capped: boolean;
    }>;
    if (rows.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C4: pil_score_delta distribution + cap-bind rate < 25%',
        detail: 'No founder PIL audits yet — C2 must PASS first.'
      });
    } else {
      const deltas = rows.map((r) => Math.abs(Number(r.pil_score_delta))).sort((a, b) => a - b);
      const cappedCount = rows.filter((r) => r.pil_score_delta_was_capped === true).length;
      const capRate = cappedCount / rows.length;
      const absMean = deltas.reduce((a, b) => a + b, 0) / deltas.length;
      const distMsg =
        `n=${rows.length} |Δ| min=${quantile(deltas, 0).toFixed(3)} ` +
        `p50=${quantile(deltas, 0.5).toFixed(3)} ` +
        `p90=${quantile(deltas, 0.9).toFixed(3)} ` +
        `max=${quantile(deltas, 1).toFixed(3)} ` +
        `abs-mean=${absMean.toFixed(3)} ` +
        `cap-bind rate=${(capRate * 100).toFixed(1)}%`;
      if (capRate <= CAP_BIND_RATE_WARN_THRESHOLD) {
        findings.push({
          severity: 'pass',
          criterion: 'C4: pil_score_delta distribution + cap-bind rate < 25%',
          detail: `${distMsg}. Cap is a guardrail, not the operating point.`
        });
      } else {
        findings.push({
          severity: 'warn',
          criterion: 'C4: pil_score_delta distribution + cap-bind rate < 25%',
          detail:
            `${distMsg}. Cap-bind rate exceeds 25% — model may be leaning hard on the prior. ` +
            `Review the §17 founder observations; consider whether FOMO_PIL_SCORE_CAP tightening or prompt-tune is warranted in a future phase.`
        });
      }
    }
  }

  /* ------------------------------------------------------------------ */
  /* C5: Privacy canary 0 hits on PIL audit detail                      */
  /* ------------------------------------------------------------------ */

  if (!pilAppliedRegistered || !CANARY_OPEN_TS) {
    findings.push({
      severity: 'pending',
      criterion: 'C5: Privacy canary — 0 hits on PIL audit detail',
      detail: 'Pending v0.5.12 substrate + canary open timestamp.'
    });
  } else {
    const auditDetails = await db.execute<{ detail_text: string }>(
      sql`SELECT detail::text AS detail_text FROM audit_log
          WHERE action = ${PIL_APPLIED_KIND}
            AND occurred_at >= ${CANARY_OPEN_TS}::timestamptz`
    );
    const rows = auditDetails.rows as Array<{ detail_text: string }>;
    let canaryHits = 0;
    const hitSamples: string[] = [];
    for (const r of rows) {
      const lower = r.detail_text.toLowerCase();
      for (const needle of FORBIDDEN_DETAIL_SUBSTRINGS) {
        if (lower.includes(needle.toLowerCase())) {
          canaryHits++;
          if (hitSamples.length < 3) hitSamples.push(needle);
          break;
        }
      }
    }
    if (canaryHits === 0) {
      findings.push({
        severity: 'pass',
        criterion: 'C5: Privacy canary — 0 hits on PIL audit detail',
        detail: `scanned ${rows.length} brevio.rank.pil_applied row(s) against ${FORBIDDEN_DETAIL_SUBSTRINGS.length} forbidden substring(s); zero hits.`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C5: Privacy canary — 0 hits on PIL audit detail',
        detail:
          `${canaryHits} row(s) of ${rows.length} contain forbidden substring(s). Sample needles: ${hitSamples.join(', ')}. ` +
          `Inspect: SELECT id, occurred_at, detail FROM audit_log WHERE action='${PIL_APPLIED_KIND}' AND occurred_at >= '${CANARY_OPEN_TS}'.`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C6 LOAD-BEARING: cross-user contamination — 0 founder audits cite a  */
  /* scope_key without a founder memory_signal row                       */
  /* ------------------------------------------------------------------ */

  if (!pilAppliedRegistered || !CANARY_OPEN_TS) {
    findings.push({
      severity: 'pending',
      criterion: 'C6 (LOAD-BEARING): Cross-user contamination — 0 founder PIL audits cite a scope_key without a founder memory_signal row',
      detail: 'Pending v0.5.12 substrate + canary open timestamp.'
    });
  } else {
    const contam = await db.execute<{ n: string }>(
      sql`SELECT COUNT(*)::text AS n FROM audit_log a
          WHERE a.action = ${PIL_APPLIED_KIND}
            AND a.actor_user_id = ${FOUNDER_USER_ID}
            AND a.occurred_at >= ${CANARY_OPEN_TS}::timestamptz
            AND NOT EXISTS (
              SELECT 1 FROM memory_signals m
              WHERE m.user_id = ${FOUNDER_USER_ID}
                AND m.scope_key = a.detail->>'scope_key_hash'
                AND m.kind IN ('sender_importance', 'sender_suppressed')
            )`
    );
    const n = Number((contam.rows[0] as { n: string } | undefined)?.n ?? '0');
    if (n === 0) {
      findings.push({
        severity: 'pass',
        criterion: 'C6 (LOAD-BEARING): Cross-user contamination — 0 founder PIL audits cite a scope_key without a founder memory_signal row',
        detail: `0 contaminating row(s). HMAC user_id construction architectural; allowlist gate confirmed in vivo.`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C6 (LOAD-BEARING): Cross-user contamination — 0 founder PIL audits cite a scope_key without a founder memory_signal row',
        detail:
          `${n} contaminating row(s). Either the read-side filter regressed or HMAC scope_key generation drifted. ` +
          `Inspect: SELECT id, occurred_at, detail->>'scope_key_hash' FROM audit_log WHERE action='${PIL_APPLIED_KIND}' AND actor_user_id='${FOUNDER_USER_ID}' AND occurred_at >= '${CANARY_OPEN_TS}'.`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C7 LOAD-BEARING: Soft rollback in vivo                             */
  /* ------------------------------------------------------------------ */

  if (!pilAppliedRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C7 (LOAD-BEARING): Soft rollback (clear allowlist mid-canary) → next fresh rank ranker-v0.2.0 + no new PIL audit',
      detail: 'Pending v0.5.12 substrate.'
    });
  } else if (!SOFT_ROLLBACK_TS) {
    findings.push({
      severity: 'pending',
      criterion: 'C7 (LOAD-BEARING): Soft rollback (clear allowlist mid-canary) → next fresh rank ranker-v0.2.0 + no new PIL audit',
      detail:
        `FOMO_V0_5_13_SOFT_ROLLBACK_TS unset. Set this env var to the timestamp when the operator cleared ` +
        `FOMO_PIL_LIVE_USER_ALLOWLIST and restarted the server, so this check scopes to the post-rollback window.`
    });
  } else {
    const auditsAfter = await db.execute<{ n: string }>(
      sql`SELECT COUNT(*)::text AS n FROM audit_log
          WHERE action = ${PIL_APPLIED_KIND}
            AND actor_user_id = ${FOUNDER_USER_ID}
            AND occurred_at > ${SOFT_ROLLBACK_TS}::timestamptz
            AND (
              ${HARD_ROLLBACK_TS} = ''
              OR occurred_at < ${HARD_ROLLBACK_TS}::timestamptz
            )`
    );
    const auditN = Number((auditsAfter.rows[0] as { n: string } | undefined)?.n ?? '0');
    const ranksAfter = await db.execute<{ prompt_version: string }>(
      sql`SELECT prompt_version FROM rank_results
          WHERE user_id = ${FOUNDER_USER_ID}
            AND created_at > ${SOFT_ROLLBACK_TS}::timestamptz
            AND (
              ${HARD_ROLLBACK_TS} = ''
              OR created_at < ${HARD_ROLLBACK_TS}::timestamptz
            )
          ORDER BY created_at DESC`
    );
    const ranks = ranksAfter.rows as Array<{ prompt_version: string }>;
    if (auditN === 0 && ranks.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C7 (LOAD-BEARING): Soft rollback (clear allowlist mid-canary) → next fresh rank ranker-v0.2.0 + no new PIL audit',
        detail:
          `Soft rollback flip at ${SOFT_ROLLBACK_TS} but no fresh founder rank yet observed. ` +
          `Drive one more rank (natural Gmail or synthetic-seed) post-rollback to complete this check.`
      });
    } else if (auditN === 0 && ranks.every((r) => r.prompt_version === EXPECTED_V0511_RANKER_BASELINE)) {
      findings.push({
        severity: 'pass',
        criterion: 'C7 (LOAD-BEARING): Soft rollback (clear allowlist mid-canary) → next fresh rank ranker-v0.2.0 + no new PIL audit',
        detail:
          `Post-rollback window: 0 new PIL audits + ${ranks.length} fresh rank(s) all prompt_version='${EXPECTED_V0511_RANKER_BASELINE}'. ` +
          `Allowlist-clear rollback works as expected.`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C7 (LOAD-BEARING): Soft rollback (clear allowlist mid-canary) → next fresh rank ranker-v0.2.0 + no new PIL audit',
        detail:
          `Post-rollback window: ${auditN} new PIL audit(s), ${ranks.length} fresh rank(s); ` +
          `${ranks.filter((r) => r.prompt_version === EXPECTED_V0512_RANKER_PIL).length} have prompt_version='${EXPECTED_V0512_RANKER_PIL}'. ` +
          `Allowlist gate did not stop PIL evaluation — investigate the worker-level check.`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C8 LOAD-BEARING: Hard rollback in vivo                             */
  /* ------------------------------------------------------------------ */

  if (!pilAppliedRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C8 (LOAD-BEARING): Hard rollback (FOMO_PIL_LIVE_ENABLED=false mid-canary) → next fresh rank ranker-v0.2.0 + no new PIL audit',
      detail: 'Pending v0.5.12 substrate.'
    });
  } else if (!HARD_ROLLBACK_TS) {
    findings.push({
      severity: 'pending',
      criterion: 'C8 (LOAD-BEARING): Hard rollback (FOMO_PIL_LIVE_ENABLED=false mid-canary) → next fresh rank ranker-v0.2.0 + no new PIL audit',
      detail:
        `FOMO_V0_5_13_HARD_ROLLBACK_TS unset. Set this env var to the timestamp when the operator flipped ` +
        `FOMO_PIL_LIVE_ENABLED=false and restarted the server.`
    });
  } else {
    const auditsAfter = await db.execute<{ n: string }>(
      sql`SELECT COUNT(*)::text AS n FROM audit_log
          WHERE action = ${PIL_APPLIED_KIND}
            AND actor_user_id = ${FOUNDER_USER_ID}
            AND occurred_at > ${HARD_ROLLBACK_TS}::timestamptz`
    );
    const auditN = Number((auditsAfter.rows[0] as { n: string } | undefined)?.n ?? '0');
    const ranksAfter = await db.execute<{ prompt_version: string }>(
      sql`SELECT prompt_version FROM rank_results
          WHERE user_id = ${FOUNDER_USER_ID}
            AND created_at > ${HARD_ROLLBACK_TS}::timestamptz
          ORDER BY created_at DESC`
    );
    const ranks = ranksAfter.rows as Array<{ prompt_version: string }>;
    if (auditN === 0 && ranks.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C8 (LOAD-BEARING): Hard rollback (FOMO_PIL_LIVE_ENABLED=false mid-canary) → next fresh rank ranker-v0.2.0 + no new PIL audit',
        detail:
          `Hard rollback flip at ${HARD_ROLLBACK_TS} but no fresh founder rank yet observed. ` +
          `Drive one more rank post-rollback to complete this check.`
      });
    } else if (auditN === 0 && ranks.every((r) => r.prompt_version === EXPECTED_V0511_RANKER_BASELINE)) {
      findings.push({
        severity: 'pass',
        criterion: 'C8 (LOAD-BEARING): Hard rollback (FOMO_PIL_LIVE_ENABLED=false mid-canary) → next fresh rank ranker-v0.2.0 + no new PIL audit',
        detail:
          `Post-rollback window: 0 new PIL audits + ${ranks.length} fresh rank(s) all prompt_version='${EXPECTED_V0511_RANKER_BASELINE}'. ` +
          `Global kill switch rollback works as expected.`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C8 (LOAD-BEARING): Hard rollback (FOMO_PIL_LIVE_ENABLED=false mid-canary) → next fresh rank ranker-v0.2.0 + no new PIL audit',
        detail:
          `Post-rollback window: ${auditN} new PIL audit(s), ${ranks.length} fresh rank(s); ` +
          `${ranks.filter((r) => r.prompt_version === EXPECTED_V0512_RANKER_PIL).length} have prompt_version='${EXPECTED_V0512_RANKER_PIL}'. ` +
          `Global kill switch did not stop PIL evaluation — boot wiring regression.`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C9: Legacy message:<id> rows still ignored                          */
  /* ------------------------------------------------------------------ */

  if (!pilAppliedRegistered || !CANARY_OPEN_TS) {
    findings.push({
      severity: 'pending',
      criterion: 'C9: Legacy message:<id> rows still ignored at read side',
      detail: 'Pending v0.5.12 substrate + canary open timestamp.'
    });
  } else {
    const legacyAudits = await db.execute<{ n: string }>(
      sql`SELECT COUNT(*)::text AS n FROM audit_log
          WHERE action = ${PIL_APPLIED_KIND}
            AND occurred_at >= ${CANARY_OPEN_TS}::timestamptz
            AND detail->>'scope_key_hash' LIKE 'message:%'`
    );
    const n = Number((legacyAudits.rows[0] as { n: string } | undefined)?.n ?? '0');
    const legacyRows = await db.execute<{ n: string }>(
      sql`SELECT COUNT(*)::text AS n FROM memory_signals
          WHERE kind = 'sender_suppressed' AND scope_key LIKE 'message:%'`
    );
    const legacyN = Number((legacyRows.rows[0] as { n: string } | undefined)?.n ?? '0');
    if (n === 0) {
      findings.push({
        severity: 'pass',
        criterion: 'C9: Legacy message:<id> rows still ignored at read side',
        detail: `${legacyN} legacy message:* placeholder row(s) present in memory_signals; 0 PIL audits cite them. Read-side filter intact.`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C9: Legacy message:<id> rows still ignored at read side',
        detail: `${n} PIL audit(s) cite a legacy message:* scope_key — read-side filter regression.`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C10: v0.5.11 substrate remains unchanged                            */
  /* ------------------------------------------------------------------ */

  if (v0511SubstrateIntact) {
    const aggCount = await db.execute<{ n: string }>(
      sql`SELECT COUNT(*)::text AS n FROM audit_log WHERE action = 'brevio.signal.aggregated'`
    );
    const aggN = Number((aggCount.rows[0] as { n: string } | undefined)?.n ?? '0');
    findings.push({
      severity: 'pass',
      criterion: 'C10: v0.5.11 substrate remains unchanged',
      detail:
        `Registry + module + column checks PASS. ${aggN} brevio.signal.aggregated audit(s) total. ` +
        `Operator runs smoke-evidence:v0.5.11 as carry-forward; expects VERDICT: PASS.`
    });
  } else {
    const missing: string[] = [];
    if (!pilKindsRegistered) missing.push('PIL kinds');
    if (!signalAggregatedRegistered) missing.push('brevio.signal.aggregated');
    if (!alertsHashColumnPresent) missing.push('alerts.sender_email_hash column');
    findings.push({
      severity: 'fail',
      criterion: 'C10: v0.5.11 substrate remains unchanged',
      detail: `Missing prerequisites: ${missing.join(', ')}`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C11: v0.5.9 + v0.5.10 + v0.5.12 carry-forward                       */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: 'pending',
    criterion: 'C11: v0.5.9 + v0.5.10 + v0.5.12 carry-forward smoke-evidence PASS',
    detail:
      `OPERATOR MUST RUN: pnpm smoke-evidence:v0.5.9 + smoke-evidence:v0.5.10 + smoke-evidence:v0.5.12. ` +
      `Expected: all PASS or match documented benign shapes per [[v05-12-pass]] / [[v05-10-pass]] / [[v05-9-pass]].`
  });

  /* ------------------------------------------------------------------ */
  /* C12: v0.5.12 BB1-BB8 deterministic regression                       */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: 'pending',
    criterion: 'C12: All v0.5.12 BB1-BB8 still PASS deterministically (pil-live.eval.ts)',
    detail:
      `OPERATOR MUST RUN: pnpm --filter @brevio/fomo run eval:pil-live ×3. ` +
      `Expected: VERDICT PASS on all 3 runs, all 19 fixtures green every run. ` +
      `Any flake or fail on BB1-BB8 is a v0.5.13 regression of the v0.5.12 contract.`
  });

  /* ------------------------------------------------------------------ */
  /* HMR + reply parser + ranker prompt-version carry-forward (sanity)  */
  /* ------------------------------------------------------------------ */

  if (hmrUnchanged && replyParserUnchanged && rankerPromptVersionValid) {
    findings.push({
      severity: 'pass',
      criterion: 'Carry-forward sanity: HMR + reply parser + ranker PROMPT_VERSION unchanged',
      detail:
        `HMR template='${FOUNDER_TEXT_TEMPLATE_VERSION}', reply-parser='${REPLY_PARSER_PROMPT_VERSION}', ` +
        `ranker='${rankerPromptVersion}'.`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'Carry-forward sanity: HMR + reply parser + ranker PROMPT_VERSION unchanged',
      detail:
        `HMR='${FOUNDER_TEXT_TEMPLATE_VERSION}' (expected '${EXPECTED_HMR_TEMPLATE_VERSION}'); ` +
        `reply-parser='${REPLY_PARSER_PROMPT_VERSION}' (expected '${EXPECTED_V0510_PROMPT_VERSION}'); ` +
        `ranker='${rankerPromptVersion}' (expected '${EXPECTED_V0511_RANKER_BASELINE}' or '${EXPECTED_V0512_RANKER_PIL}').`
    });
  }

  /* ============================================================== */
  /* Summary                                                         */
  /* ============================================================== */

  console.log('========================================================================');
  console.log('Phase v0.5.13 evidence summary — 12 criteria (founder-only PIL live canary)');
  console.log('========================================================================');

  let passCount = 0;
  let warnCount = 0;
  let failCount = 0;
  let pendingCount = 0;

  for (const f of findings) {
    console.log(`  [${symbol(f.severity)}] ${f.criterion}`);
    console.log(`        ${f.detail}`);
    if (f.severity === 'pass') passCount++;
    else if (f.severity === 'warn') warnCount++;
    else if (f.severity === 'fail') failCount++;
    else pendingCount++;
  }

  console.log('');

  if (runtimePending) {
    console.log(
      `VERDICT: SCAFFOLDING — runtime commit(s) pending. ` +
        `${passCount} PASS, ${pendingCount} PENDING, ${warnCount} WARN, ${failCount} FAIL.`
    );
    process.exit(0);
  }

  if (failCount > 0) {
    console.log(
      `VERDICT: FAIL  — ${failCount} criterion(criteria) failed. ${passCount} PASS, ${pendingCount} PENDING, ${warnCount} WARN.`
    );
    process.exit(1);
  }

  console.log(
    `VERDICT: PASS  (${passCount} PASS, ${pendingCount} operator-confirmed, ${warnCount} warn). ` +
      `Operator must additionally run: smoke-evidence:v0.5.9 + smoke-evidence:v0.5.10 + smoke-evidence:v0.5.11 + smoke-evidence:v0.5.12 (carry-forward) ` +
      `+ \`pnpm --filter @brevio/fomo run eval:pil-live\` (C12 regression — must PASS 3/3 deterministically).`
  );
}

main().catch((err) => {
  process.stderr.write(`[smoke-evidence:v0.5.13] crashed: ${String(err)}\n`);
  process.exit(2);
});
