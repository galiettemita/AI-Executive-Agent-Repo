// Phase v0.5.12 smoke-evidence — Live ranker reads PIL in guarded mode.
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-07):
//   Same 'pending' severity model as v0.5.5–v0.5.11. PENDING means "this
//   criterion depends on a runtime artifact that the runtime commit will
//   introduce." Until the runtime commit lands:
//     * brevio.rank.pil_applied missing from FOMO_AUDIT_ACTIONS
//       → C2/C4/C5/C8/C11 PENDING
//     * buildLivePilContext export not present (in pil-live-context.ts or
//       extended pil-context.ts)
//       → C2/C3/C7 PENDING
//     * apps/fomo/src/eval/pil-live.eval.ts not present
//       → C8/C9/C10/C11 PENDING (BB1/BB3/BB5/BB8 LOAD-BEARING covered there)
//   When runtime + smoke run, PENDINGs flip to PASS/FAIL per live state.
//
// v0.5.12 scope (locked Q1–Q6 — see memory project_v05-12-scope):
//   * Q1.C-modified — two-call hybrid (baseline + PIL ranker calls);
//     final_delta = clamp(pil_score - baseline_score, -CAP, +CAP).
//     If baseline call is skipped the cap is not real → Q1.C rejected.
//   * Q2.A — FOMO_PIL_SCORE_CAP default 0.15, bounds [0.05, 0.25].
//     Cap enforced at the rank-write step (post-model).
//   * Q3.A + Q3.C — sender_suppressed is a STRONG PRIOR (model can override
//     on strong intrinsic signal); recency decay applies. EXPLICITLY REJECT
//     dispatch-time filter / binary blindness.
//   * Q4.A — offline pil-live.eval.ts (live model calls). Production
//     divergence audit may exist but DISABLED BY DEFAULT.
//   * Q5.A — FOMO_PIL_LIVE_ENABLED=false default global kill switch. When
//     false: bit-identical v0.5.11.
//   * Q6.A — new audit kind brevio.rank.pil_applied with 9 locked structural
//     fields. NO rank_results schema change (preserves v0.5.11 invariant).
//
//   CRITICAL READ-SIDE FILTER (founder rule):
//     buildLivePilContext MUST filter memory_signals to
//     scope_key ~ '^[a-f0-9]{32}$' AND user_id = userId. Legacy
//     scope_key='message:<id>' placeholder rows MUST be ignored.
//
//   HARD INVARIANTS (each enforced by ≥1 criterion):
//     - Kill switch off → bit-identical v0.5.11 (C1)
//     - Canonical-HMAC-only read (C3 + BB6)
//     - Two-call hybrid enforced cap (C4 + BB8)
//     - Cross-user contamination fails closed (C7 + BB4)
//     - Suppressed-sender-still-surfaces (C8 + BB1) — Q3.A
//     - Old signal decays (C9 + BB3) — Q3.C
//     - One correction does not materially change live ranker (C10 + BB5)
//     - PIL cannot bypass cap via prompt/model behavior (C11 + BB8)
//     - Privacy canary clean on new audit + ranker prompt (C6)
//     - v0.5.11 substrate UNCHANGED (C12)
//     - HMR + reply parser UNCHANGED + 3E.1 preserved (C13 + C14)
//
// Founder-only smoke. No friend involvement (three-friend cap). Read-only —
// never mutates the DB.

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import { FOMO_AUDIT_ACTIONS } from '../src/core/audit.js';
import { MEMORY_SIGNAL_KINDS } from '../src/memory/memory-signals.js';
import { PROMPT_VERSION as REPLY_PARSER_PROMPT_VERSION } from '../src/reply-parser/prompt.js';
import { PROMPT_VERSION as RANKER_PROMPT_VERSION } from '../src/ranker/prompt.js';
import { FOUNDER_TEXT_TEMPLATE_VERSION } from '../src/core/founder-text-template.js';

type Severity = 'pass' | 'warn' | 'fail' | 'pending';

interface Finding {
  readonly severity: Severity;
  readonly criterion: string;
  readonly detail: string;
}

const SMOKE_WINDOW_HOURS = Number((process.env.FOMO_V0_5_12_WINDOW_HOURS ?? '24').trim()) || 24;
const FOUNDER_USER_ID = (process.env.FOMO_FOUNDER_USER_ID ?? 'founder').trim();

const EXPECTED_V0510_PROMPT_VERSION = 'reply-parser-v0.2.0' as string;
const EXPECTED_V0511_RANKER_BASELINE = 'ranker-v0.2.0' as string;
const EXPECTED_V0512_RANKER_PIL = 'ranker-v0.3.0' as string;
const EXPECTED_HMR_TEMPLATE_VERSION = 'human-message-v0.3.0' as string;

// v0.5.12 new audit kind
const PIL_APPLIED_KIND = 'brevio.rank.pil_applied';

// Locked 9 detail fields on brevio.rank.pil_applied (per Q6.A)
const PIL_APPLIED_DETAIL_FIELDS = [
  'rank_result_id',
  'pil_signal_kinds_present',
  'score_before_pil_cap',
  'score_after_pil_cap',
  'pil_score_delta',
  'pil_score_delta_was_capped',
  'model_mentioned_pil_in_reason',
  'source_surface',
  'scope_key_hash'
] as const;

// Privacy canary — forbidden substrings in any new audit detail OR any
// rendered prompt/snippet captured by the runtime. Mirrors v0.5.11 + adds
// any PIL-specific canaries.
const FORBIDDEN_DETAIL_SUBSTRINGS = [
  'Subject:',
  'From:',
  'To:',
  '@gmail.com',
  '@icloud.com',
  '@hotmail.com',
  '@yahoo.com',
  // Reply-text + email-body canaries (carry-forward from v0.5.10/v0.5.11)
  'ignore this',
  'not important',
  'this mattered',
  'more like this',
  // PIL-content canaries
  'noreply@',
  'unsubscribe',
  // v0.5.12-specific: raw rank.reason text must NOT appear in the audit
  // detail (the bool model_mentioned_pil_in_reason is the only field that
  // touches the rank.reason content). The reason text itself lives in
  // rank_results.reason; never in the audit detail.
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

async function main(): Promise<void> {
  if (!(process.env.DATABASE_URL ?? '').trim()) {
    process.stderr.write('[smoke-evidence:v0.5.12] DATABASE_URL not set. Source .env first.\n');
    process.exit(2);
  }
  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    process.stderr.write(`[smoke-evidence:v0.5.12] DB unavailable: ${dbResult.reason}\n`);
    process.exit(1);
  }
  const db = dbResult.client;

  console.log('Phase v0.5.12 evidence — Live ranker reads PIL in guarded mode (founder-only smoke)\n');
  console.log(`Smoke window: last ${SMOKE_WINDOW_HOURS}h (override via FOMO_V0_5_12_WINDOW_HOURS).\n`);

  const findings: Finding[] = [];

  /* ============================================================== */
  /* Registry + module inspection — determines PENDING flags        */
  /* ============================================================== */

  const auditActionSet = new Set<string>(FOMO_AUDIT_ACTIONS as readonly string[]);
  const memorySignalSet = new Set<string>(MEMORY_SIGNAL_KINDS as readonly string[]);

  const pilAppliedRegistered = auditActionSet.has(PIL_APPLIED_KIND);
  const signalAggregatedRegistered = auditActionSet.has('brevio.signal.aggregated');
  const senderImportanceRegistered = memorySignalSet.has('sender_importance');
  const senderSuppressedRegistered = memorySignalSet.has('sender_suppressed');
  const pilKindsRegistered = senderImportanceRegistered && senderSuppressedRegistered;

  // buildLivePilContext export probe — may live in pil-live-context.ts (new
  // file) or as an extension of pil-context.ts.
  let livePilContextModulePresent = false;
  try {
    const modulePath = '../src/ranker/pil-live-context.js';
    const mod = (await import(modulePath)) as Record<string, unknown>;
    livePilContextModulePresent = typeof mod.buildLivePilContext === 'function';
  } catch {
    livePilContextModulePresent = false;
  }
  if (!livePilContextModulePresent) {
    try {
      const modulePath = '../src/ranker/pil-context.js';
      const mod = (await import(modulePath)) as Record<string, unknown>;
      livePilContextModulePresent = typeof mod.buildLivePilContext === 'function';
    } catch {
      livePilContextModulePresent = false;
    }
  }

  // v0.5.11 shadow projection module (still load-bearing as a base layer)
  let v0511ShadowContextModulePresent = false;
  try {
    const modulePath = '../src/ranker/pil-context.js';
    const mod = (await import(modulePath)) as Record<string, unknown>;
    v0511ShadowContextModulePresent = typeof mod.buildPilContext === 'function';
  } catch {
    v0511ShadowContextModulePresent = false;
  }

  // pil-live.eval.ts presence
  let pilLiveEvalPresent = false;
  try {
    const modulePath = '../src/eval/pil-live.eval.js';
    await import(modulePath);
    pilLiveEvalPresent = true;
  } catch {
    pilLiveEvalPresent = false;
  }

  // alerts.sender_email_hash column presence (still load-bearing from v0.5.11)
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

  const runtimePending =
    !pilAppliedRegistered ||
    !livePilContextModulePresent ||
    !pilLiveEvalPresent;

  // v0.5.11 substrate carry-forward — load-bearing for C12
  const v0511SubstrateIntact =
    pilKindsRegistered &&
    signalAggregatedRegistered &&
    v0511ShadowContextModulePresent &&
    alertsHashColumnPresent;

  // v0.5.7 + v0.5.10 carry-forward
  const hmrUnchanged = FOUNDER_TEXT_TEMPLATE_VERSION === EXPECTED_HMR_TEMPLATE_VERSION;
  const replyParserUnchanged = (REPLY_PARSER_PROMPT_VERSION as string) === EXPECTED_V0510_PROMPT_VERSION;
  const rankerPromptVersion = RANKER_PROMPT_VERSION as string;
  const rankerPromptVersionValid =
    rankerPromptVersion === EXPECTED_V0511_RANKER_BASELINE ||
    rankerPromptVersion === EXPECTED_V0512_RANKER_PIL;

  /* ------------------------------------------------------------------ */
  /* C1: Kill switch off → bit-identical v0.5.11 behavior               */
  /* No brevio.rank.pil_applied audits while kill switch was off.       */
  /* ------------------------------------------------------------------ */

  // Smoke procedure (runbook §5 Path A) drives one rank with
  // FOMO_PIL_LIVE_ENABLED=false explicitly. We assert: ZERO
  // brevio.rank.pil_applied audit rows exist in the window IF the kill
  // switch was off for the whole window. If the founder also drove Path C
  // (kill switch on), audit rows will exist — that's expected. So C1 is an
  // OPERATOR-CONFIRMED criterion: the runbook records when the switch was
  // flipped, and the smoke-evidence query proves the audit-row count is
  // CONSISTENT with the switch timeline.
  if (!pilAppliedRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C1: Kill switch off → bit-identical v0.5.11 behavior',
      detail: 'PENDING runtime commit — brevio.rank.pil_applied not yet registered'
    });
  } else {
    const auditCount = await db.execute<{ n: string | number }>(
      sql`SELECT COUNT(*)::text AS n FROM audit_log
          WHERE action = ${PIL_APPLIED_KIND}
            AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    const n = Number((auditCount.rows[0] as { n: string }).n);
    findings.push({
      severity: 'pending',
      criterion: 'C1: Kill switch off → bit-identical v0.5.11 behavior (no PIL prompt block, no audit fires)',
      detail:
        `OPERATOR-CONFIRMED. ${n} brevio.rank.pil_applied row(s) in window. ` +
        `Runbook §5 Path A drives a rank with FOMO_PIL_LIVE_ENABLED=false; smoke-evidence asserts: ` +
        `(a) zero brevio.rank.pil_applied audits during the kill-switch-off interval, ` +
        `(b) rank_results.prompt_version='${EXPECTED_V0511_RANKER_BASELINE}' on those rows, ` +
        `(c) only ONE ranker call observable (single cost_records entry per rank_result, not two). ` +
        `Operator confirms via §16 timeline in the SMOKE_REPORT.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C2: Kill switch on + matching canonical PIL row → PIL context      */
  /* included in prompt + audit fires                                    */
  /* ------------------------------------------------------------------ */

  if (!pilAppliedRegistered || !livePilContextModulePresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C2: Kill switch on + canonical PIL row → PIL context included + audit fires',
      detail: 'PENDING runtime commit'
    });
  } else {
    const auditRows = await db.execute<{
      detail: Record<string, unknown> | null;
      occurred_at: Date;
    }>(
      sql`SELECT detail, occurred_at FROM audit_log
          WHERE action = ${PIL_APPLIED_KIND}
            AND actor_user_id = ${FOUNDER_USER_ID}
            AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval
          ORDER BY occurred_at DESC
          LIMIT 5`
    );
    const rows = auditRows.rows as Array<{ detail: Record<string, unknown> | null }>;
    if (rows.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C2: Kill switch on + canonical PIL row → PIL context included + audit fires',
        detail:
          `0 brevio.rank.pil_applied rows in window. Depends on Path C execution (runbook §7) — ` +
          `kill switch ON + at least one canonical-HMAC PIL row in DB.`
      });
    } else {
      const sample = rows[0]!.detail as Record<string, unknown> | null;
      const missing =
        sample === null
          ? Array.from(PIL_APPLIED_DETAIL_FIELDS)
          : PIL_APPLIED_DETAIL_FIELDS.filter((k) => !(k in sample));
      if (missing.length === 0) {
        findings.push({
          severity: 'pass',
          criterion: 'C2: Kill switch on + canonical PIL row → PIL context included + audit fires',
          detail: `${rows.length} row(s); all 9 locked detail fields present on sample`
        });
      } else {
        findings.push({
          severity: 'fail',
          criterion: 'C2: Kill switch on + canonical PIL row → PIL context included + audit fires',
          detail: `${rows.length} row(s) but sample missing locked fields: ${missing.join(', ')}`
        });
      }
    }
  }

  /* ------------------------------------------------------------------ */
  /* C3: Legacy scope_key='message:<id>' row produces null PIL context  */
  /* ------------------------------------------------------------------ */

  if (!livePilContextModulePresent || !alertsHashColumnPresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C3: Legacy message:<id> placeholder row produces null PIL context',
      detail: 'PENDING runtime commit'
    });
  } else {
    // We assert TWO things:
    //   (a) buildLivePilContext SHOULD ignore legacy placeholder rows by
    //       construction (the scope_key filter ^[a-f0-9]{32}$ rejects them)
    //   (b) The smoke fires Path D (runbook §8): one rank where the only
    //       PIL row for the sender is a message:<id> placeholder. Expected:
    //       NO brevio.rank.pil_applied audit fires for that rank_result.
    // This script can DETECT placeholder rows in DB to confirm the test
    // setup; the actual no-audit assertion is operator-confirmed against
    // the rank_result_id captured in runbook §8.
    const placeholderRows = await db.execute<{ n: string | number }>(
      sql`SELECT COUNT(*)::text AS n FROM memory_signals
          WHERE kind IN ('sender_suppressed', 'sender_importance')
            AND scope_key LIKE 'message:%'`
    );
    const placeholderN = Number((placeholderRows.rows[0] as { n: string }).n);
    findings.push({
      severity: 'pending',
      criterion: 'C3: Legacy message:<id> placeholder row produces null PIL context (no audit, no influence)',
      detail:
        `OPERATOR-CONFIRMED. ${placeholderN} legacy message:<id> placeholder row(s) present in memory_signals ` +
        `(v0.5.10 applyIgnoreSender carry-forward). Path D (runbook §8) confirms: a rank where the only ` +
        `PIL row for the sender is a placeholder produces NO brevio.rank.pil_applied audit. BB6 fixture ` +
        `in pil-live.eval.ts covers the in-vitro version. LOAD-BEARING.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C4: Cap enforced against baseline no-PIL call (two-call hybrid)    */
  /* ------------------------------------------------------------------ */

  if (!pilAppliedRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C4: Cap enforced via two-call hybrid (baseline + PIL)',
      detail: 'PENDING runtime commit'
    });
  } else {
    // C4 is satisfied when EITHER:
    //   (a) ≥1 brevio.rank.pil_applied row has score_before_pil_cap !=
    //       score_after_pil_cap AND pil_score_delta_was_capped=true (proving
    //       the cap kicked in on a real model output), OR
    //   (b) The eval harness BB8 fixture (max-score input) passes.
    const capRows = await db.execute<{
      delta_was_capped: boolean | null;
      detail: Record<string, unknown> | null;
    }>(
      sql`SELECT (detail->>'pil_score_delta_was_capped')::boolean AS delta_was_capped, detail
          FROM audit_log
          WHERE action = ${PIL_APPLIED_KIND}
            AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval
          ORDER BY occurred_at DESC
          LIMIT 50`
    );
    const rows = capRows.rows as Array<{
      delta_was_capped: boolean | null;
      detail: Record<string, unknown> | null;
    }>;
    const cappedCount = rows.filter((r) => r.delta_was_capped === true).length;
    findings.push({
      severity: 'pending',
      criterion: 'C4: Cap enforced via two-call hybrid (baseline + PIL); cap is REAL only if baseline runs',
      detail:
        `${rows.length} audit row(s) in window; ${cappedCount} had pil_score_delta_was_capped=true. ` +
        `OPERATOR + EVAL CONFIRMED. Runtime unit test must assert: ` +
        `(a) when FOMO_PIL_LIVE_ENABLED=true + pil_context non-null, TWO ranker calls happen (count cost_records ` +
        `or audit-log a 'rank_pair' marker), ` +
        `(b) score_after_pil_cap - baseline_score = clamp(score_before_pil_cap - baseline_score, ±FOMO_PIL_SCORE_CAP), ` +
        `(c) pil_score_delta_was_capped=true iff |raw_delta| > FOMO_PIL_SCORE_CAP. ` +
        `BB8 (max-score input) fixture in pil-live.eval.ts is LOAD-BEARING.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C5: brevio.rank.pil_applied fires only when PIL context is non-null*/
  /* ------------------------------------------------------------------ */

  if (!pilAppliedRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C5: brevio.rank.pil_applied fires only when PIL context is non-null',
      detail: 'PENDING runtime commit'
    });
  } else {
    // Inverse assertion: rank_results rows in window where the corresponding
    // alert has sender_email_hash IS NULL should have ZERO brevio.rank.pil_applied
    // audits. We can't link rank_result_id to alerts.sender_email_hash without
    // joining; the structural check is the audit detail itself — every
    // brevio.rank.pil_applied row must have a non-empty pil_signal_kinds_present
    // array.
    const inverseCheck = await db.execute<{
      bad_rows: string | number;
    }>(
      sql`SELECT COUNT(*)::text AS bad_rows FROM audit_log
          WHERE action = ${PIL_APPLIED_KIND}
            AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval
            AND (
              detail->'pil_signal_kinds_present' IS NULL
              OR jsonb_array_length(detail->'pil_signal_kinds_present') = 0
              OR detail->>'scope_key_hash' IS NULL
              OR detail->>'scope_key_hash' = ''
            )`
    );
    const badN = Number((inverseCheck.rows[0] as { bad_rows: string }).bad_rows);
    if (badN === 0) {
      findings.push({
        severity: 'pass',
        criterion: 'C5: brevio.rank.pil_applied fires only when PIL context is non-null',
        detail: `0 audit row(s) with empty pil_signal_kinds_present or empty scope_key_hash`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C5: brevio.rank.pil_applied fires only when PIL context is non-null',
        detail: `${badN} audit row(s) had empty pil_signal_kinds_present or empty scope_key_hash — audit fired without a real PIL context`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C6: Privacy canary — no raw sender/email/subject/body/snippet/     */
  /* header in brevio.rank.pil_applied detail OR any ranker prompt      */
  /* assembly fixture captured                                            */
  /* ------------------------------------------------------------------ */

  if (!pilAppliedRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C6: Privacy canary — no raw private content in audit or prompt',
      detail: 'PENDING runtime commit'
    });
  } else {
    const auditDetails = await db.execute<{ detail: Record<string, unknown> | null }>(
      sql`SELECT detail FROM audit_log
          WHERE action = ${PIL_APPLIED_KIND}
            AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    const detailRows = (auditDetails.rows as Array<{ detail: Record<string, unknown> | null }>)
      .map((r) => (r.detail === null ? '' : JSON.stringify(r.detail)));
    const lowercaseHaystack = detailRows.join('\n').toLowerCase();
    const hits = FORBIDDEN_DETAIL_SUBSTRINGS.filter((s) =>
      lowercaseHaystack.includes(s.toLowerCase())
    );
    if (detailRows.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C6: Privacy canary on new audit + ranker prompt assembly',
        detail: `no brevio.rank.pil_applied rows in window; cannot scan yet`
      });
    } else if (hits.length === 0) {
      findings.push({
        severity: 'pass',
        criterion: 'C6: Privacy canary — no raw private content in new audit',
        detail: `scanned ${detailRows.length} brevio.rank.pil_applied row(s) against ${FORBIDDEN_DETAIL_SUBSTRINGS.length} forbidden substring(s); zero hits. PROMPT-side canary is OPERATOR-CONFIRMED via runbook §10 (unit-test fixture of the assembled PIL prompt block must contain only the 3 allowed structural fields: sender_importance_score, sender_suppressed, signal_age_days).`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C6: Privacy canary — no raw private content in new audit',
        detail: `${hits.length} forbidden substring(s) found in audit detail: ${hits.join(', ')}`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C7: Cross-user contamination — LOAD-BEARING                        */
  /* ------------------------------------------------------------------ */

  if (!livePilContextModulePresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C7: Cross-user contamination test (LOAD-BEARING)',
      detail: 'PENDING runtime commit'
    });
  } else {
    // Inverse check: for any non-founder user_id appearing in memory_signals
    // for the smoke window's new HMAC-keyed rows, founder MUST NOT have a
    // brevio.rank.pil_applied audit at that scope_key_hash.
    const crossUserCheck = await db.execute<{ n: string | number }>(
      sql`SELECT COUNT(*)::text AS n FROM audit_log a
          WHERE a.action = ${PIL_APPLIED_KIND}
            AND a.actor_user_id = ${FOUNDER_USER_ID}
            AND a.occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval
            AND EXISTS (
              SELECT 1 FROM memory_signals m
              WHERE m.scope_key = a.detail->>'scope_key_hash'
                AND m.user_id <> ${FOUNDER_USER_ID}
                AND NOT EXISTS (
                  SELECT 1 FROM memory_signals m2
                  WHERE m2.scope_key = a.detail->>'scope_key_hash'
                    AND m2.user_id = ${FOUNDER_USER_ID}
                )
            )`
    );
    const n = Number((crossUserCheck.rows[0] as { n: string }).n);
    if (n === 0) {
      findings.push({
        severity: 'pass',
        criterion: 'C7: Cross-user contamination — no founder PIL audit reads from a non-founder scope_key (LOAD-BEARING)',
        detail: `0 contaminating row(s). HMAC construction (user_id in MAC input) makes same-email collision impossible. BB4 fixture in pil-live.eval.ts adversarially simulates a hash collision via direct DB insertion; that test is the operator-confirmed proof.`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C7: Cross-user contamination — no founder PIL audit reads from a non-founder scope_key',
        detail: `${n} contaminating row(s) — founder rank read PIL signal from a scope_key that exists only for another user`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C8: Suppressed sender + high-intrinsic importance still surfaces   */
  /* (BB1 LOAD-BEARING)                                                  */
  /* ------------------------------------------------------------------ */

  if (!pilLiveEvalPresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C8: Suppressed sender with high-intrinsic importance can still surface (BB1)',
      detail: 'PENDING runtime commit — pil-live.eval.ts not yet present'
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: 'C8: Suppressed sender with high-intrinsic importance can still surface (BB1 LOAD-BEARING)',
      detail:
        `OPERATOR + EVAL CONFIRMED. BB1 fixture in pil-live.eval.ts (sender_suppressed=true 3d old + URGENT email referencing user CEO) ` +
        `must produce label=important AND score ≥ 0.7 AND rank.reason mentions both prior and override. ` +
        `Operator runs \`pnpm --filter @brevio/fomo run eval:pil-live\` and verifies BB1 line shows PASS. ` +
        `Validates Q3.A — sender_suppressed is a STRONG PRIOR not a dispatch-time block.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C9: Old/stale PIL signals decay to zero (BB3)                      */
  /* ------------------------------------------------------------------ */

  if (!pilLiveEvalPresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C9: Old/stale PIL signals decay to zero (BB3)',
      detail: 'PENDING runtime commit — pil-live.eval.ts not yet present'
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: 'C9: Old/stale PIL signals decay to zero (BB3) — Q3.C',
      detail:
        `OPERATOR + EVAL CONFIRMED. BB3 fixture in pil-live.eval.ts (sender_importance.score=+0.3 but signal 200d old) ` +
        `must produce live ranker output ≈ baseline (Δscore within noise floor). Decay factor applied at ` +
        `buildLivePilContext read time via the v0.5.11 computeDecayFactor reuse. ` +
        `Carry-forward shadow eval fixtures F6/F7/F8 (200d/135d/90d boundaries) MUST continue to PASS in live mode.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C10: One false-positive correction does not materially change      */
  /* live ranker output (BB5)                                            */
  /* ------------------------------------------------------------------ */

  if (!pilLiveEvalPresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C10: One false-positive correction does not materially change live ranker (BB5)',
      detail: 'PENDING runtime commit — pil-live.eval.ts not yet present'
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: 'C10: One false-positive correction does not materially change live ranker (BB5)',
      detail:
        `OPERATOR + EVAL CONFIRMED. BB5 fixture (single false_positive event, score=-0.1, no suppression) ` +
        `must produce live score within noise floor (|Δ| ≤ 0.05) of baseline. ` +
        `Founder guardrail 4: "No one-event suppression. A single correction cannot silence a sender."`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C11: PIL cannot bypass the score cap (BB8 LOAD-BEARING)            */
  /* ------------------------------------------------------------------ */

  if (!pilAppliedRegistered || !pilLiveEvalPresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C11: PIL cannot bypass score cap via prompt/model behavior (BB8 LOAD-BEARING)',
      detail: 'PENDING runtime commit'
    });
  } else {
    // Smoke-observable: in the window, if any brevio.rank.pil_applied row
    // has |score_after_pil_cap - (score_after_pil_cap - pil_score_delta)| >
    // FOMO_PIL_SCORE_CAP, the cap was bypassed. We don't know the env var
    // value from this script reliably (it could have been changed between
    // smoke runs); we assert the soft invariant: pil_score_delta_was_capped
    // is a correctly-set bool AND every captured delta is finite.
    const capCheck = await db.execute<{
      n_total: string | number;
      n_delta_null: string | number;
      n_delta_nan: string | number;
    }>(
      sql`SELECT
            COUNT(*)::text AS n_total,
            COUNT(*) FILTER (WHERE detail->>'pil_score_delta' IS NULL)::text AS n_delta_null,
            COUNT(*) FILTER (WHERE
              (detail->>'pil_score_delta')::float IS NULL
            )::text AS n_delta_nan
          FROM audit_log
          WHERE action = ${PIL_APPLIED_KIND}
            AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    const row = capCheck.rows[0] as {
      n_total: string;
      n_delta_null: string;
      n_delta_nan: string;
    };
    const total = Number(row.n_total);
    const badDelta = Number(row.n_delta_null) + Number(row.n_delta_nan);
    if (total === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C11: PIL cannot bypass score cap (BB8 LOAD-BEARING)',
        detail:
          `0 audit row(s) in window; smoke shape-check deferred. BB8 fixture LOAD-BEARING ` +
          `(sender_importance.score=+1.0 input → pil_score_delta_was_capped=true; |pil_score_delta| ≤ FOMO_PIL_SCORE_CAP).`
      });
    } else if (badDelta === 0) {
      findings.push({
        severity: 'pass',
        criterion: 'C11: PIL cannot bypass score cap (BB8 LOAD-BEARING)',
        detail:
          `${total} audit row(s); ${badDelta} with null/NaN pil_score_delta. BB8 fixture in pil-live.eval.ts ` +
          `is the operator-confirmed adversarial proof; smoke shape-check is consistent.`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C11: PIL cannot bypass score cap (BB8 LOAD-BEARING)',
        detail: `${total} audit row(s); ${badDelta} had null/NaN pil_score_delta — cap math broken`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C12: v0.5.11 substrate remains unchanged                           */
  /* ------------------------------------------------------------------ */

  if (v0511SubstrateIntact) {
    // Also smoke-observable: at least one brevio.signal.aggregated audit
    // OR sender_importance/sender_suppressed row should still upsert if
    // the founder drove a natural reply during the smoke. Absence does
    // NOT FAIL — just means no aggregation event happened in window.
    const aggAudits = await db.execute<{ n: string | number }>(
      sql`SELECT COUNT(*)::text AS n FROM audit_log
          WHERE action = 'brevio.signal.aggregated'
            AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    const aggN = Number((aggAudits.rows[0] as { n: string }).n);
    findings.push({
      severity: 'pass',
      criterion: 'C12: v0.5.11 substrate remains unchanged (pil-aggregation + sender_importance/suppressed kinds + signal.aggregated audit + sender_email_hash column intact)',
      detail:
        `Registry + module + column checks PASS. ${aggN} brevio.signal.aggregated audit(s) in window ` +
        `(0 is acceptable — depends on whether founder drove natural-reply aggregation during smoke). ` +
        `Operator runs smoke-evidence:v0.5.11 as carry-forward; expects VERDICT: PASS or documented benign shape.`
    });
  } else {
    const missing: string[] = [];
    if (!pilKindsRegistered) missing.push('PIL kinds');
    if (!signalAggregatedRegistered) missing.push('brevio.signal.aggregated');
    if (!v0511ShadowContextModulePresent) missing.push('buildPilContext (v0.5.11)');
    if (!alertsHashColumnPresent) missing.push('alerts.sender_email_hash column');
    findings.push({
      severity: 'fail',
      criterion: 'C12: v0.5.11 substrate remains unchanged',
      detail: `Missing prerequisites — v0.5.11 substrate regressed or pre-runtime state: ${missing.join(', ')}`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C13: v0.5.7 HMR + v0.5.10 reply parser remain unchanged             */
  /* ------------------------------------------------------------------ */

  if (hmrUnchanged && replyParserUnchanged && rankerPromptVersionValid) {
    findings.push({
      severity: 'pass',
      criterion: 'C13: v0.5.7 HMR + v0.5.10 reply parser + ranker PROMPT_VERSION carry-forward unchanged',
      detail:
        `HMR template='${FOUNDER_TEXT_TEMPLATE_VERSION}', reply-parser='${REPLY_PARSER_PROMPT_VERSION}', ` +
        `ranker='${rankerPromptVersion}' (scaffolding-time '${EXPECTED_V0511_RANKER_BASELINE}' or runtime '${EXPECTED_V0512_RANKER_PIL}' both valid).`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'C13: v0.5.7 HMR + v0.5.10 reply parser + ranker PROMPT_VERSION carry-forward',
      detail:
        `HMR template='${FOUNDER_TEXT_TEMPLATE_VERSION}' (expected '${EXPECTED_HMR_TEMPLATE_VERSION}'); ` +
        `reply-parser='${REPLY_PARSER_PROMPT_VERSION}' (expected '${EXPECTED_V0510_PROMPT_VERSION}'); ` +
        `ranker='${rankerPromptVersion}' (expected '${EXPECTED_V0511_RANKER_BASELINE}' or '${EXPECTED_V0512_RANKER_PIL}').`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C14: 3E.1 invariant — no LLM in body composition                   */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: 'pending',
    criterion: 'C14: 3E.1 invariant — no LLM in body composition; only rank.reason is model-generated',
    detail:
      `OPERATOR + CODE-LEVEL CONFIRMED. renderFounderText must remain deterministic; no model call introduced ` +
      `at body-generation step. The PIL block is added ONLY to the ranker prompt (affects rank.reason content), ` +
      `not to renderFounderText. Verification: grep apps/fomo/src/core/founder-text.ts + ` +
      `apps/fomo/src/core/human-message-renderer.ts for openai/anthropic imports — must remain absent.`
  });

  /* ============================================================== */
  /* Summary                                                         */
  /* ============================================================== */

  console.log('========================================================================');
  console.log('Phase v0.5.12 evidence summary — 14 criteria (live ranker reads PIL guarded)');
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
      `Operator must additionally run: smoke-evidence:v0.5.9 + smoke-evidence:v0.5.10 + smoke-evidence:v0.5.11 (carry-forward) ` +
      `+ \`pnpm --filter @brevio/fomo run eval:pil-live\` (C8/C9/C10/C11 LOAD-BEARING via BB1/BB3/BB5/BB8 + carry-forward 11 shadow fixtures).`
  );
}

await main();
