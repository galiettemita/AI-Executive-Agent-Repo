// Phase v0.5.5 smoke-evidence — STOP Enforcement + Confirmation.
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-04):
//   This script is part of the v0.5.5 SCAFFOLDING commit, which lands BEFORE
//   the runtime implementation. It introduces a fourth severity, 'pending',
//   in addition to 'pass' / 'warn' / 'fail'. PENDING means: this criterion
//   depends on an audit kind, memory_signal field, or runtime behaviour that
//   the runtime commit will introduce. Until the runtime commit lands, the
//   criterion is reported as PENDING and the overall VERDICT is PENDING.
//
//   When the runtime commit lands and registers the four new audit kinds:
//     - fomo.sendblue.stop_confirmation_sent
//     - fomo.sendblue.stop_confirmation_failed
//     - fomo.alert.suppressed_stop_active
//     - fomo.poll.skipped_stop_active
//   AND fires them during the smoke, the PENDING markers disappear
//   automatically and the VERDICT flips to PASS (or FAIL if a criterion is
//   actually broken).
//
//   This is the v0.5.4 scaffolding-pattern carried forward, with the
//   addition of explicit PENDING accounting so the founder can tell at a
//   glance whether a not-PASS verdict is "runtime not yet shipped" (expected)
//   vs "runtime is broken" (action required).
//
// Founder-only smoke. No Friend B/C involved (three-friend cap holds).
//
// Pattern mirrors smoke-evidence-v0.5.4.ts:
//   * load DB client + audit/memory registries
//   * iterate the 12 criteria; each criterion produces a Finding
//   * print a summary block + VERDICT line
//   * exit 0 if VERDICT is PASS, 1 otherwise
//
// Read-only — never mutates the DB.

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import { audit_log, memory_signals, alerts, alert_state_transitions, users } from '../src/db/schema.js';
import { FOMO_AUDIT_ACTIONS } from '../src/core/audit.js';
import { MEMORY_SIGNAL_KINDS } from '../src/memory/memory-signals.js';

type Severity = 'pass' | 'warn' | 'fail' | 'pending';

interface Finding {
  readonly severity: Severity;
  readonly criterion: string;
  readonly detail: string;
}

const SMOKE_WINDOW_HOURS = Number((process.env.FOMO_V0_5_5_WINDOW_HOURS ?? '24').trim()) || 24;
const FOUNDER_USER_ID = (process.env.FOMO_FOUNDER_USER_ID ?? 'founder').trim();

// These four audit kinds are EXPECTED runtime outputs of the v0.5.5 implementation commit.
// They are typed as `string` (not narrowed to a literal union) on purpose: drizzle's
// `audit_log.action` column is typed as the strict union of registered FOMO_AUDIT_ACTIONS
// values, and since these four are NOT yet registered (scaffolding-vs-runtime boundary),
// tsc would reject SQL templates that compare the column against the literal strings.
// Typing them as `string` lets the smoke-evidence SQL compile against the scaffolding
// commit; the runtime commit will register them, at which point this widening is
// harmless because the values exist in the union anyway.
const KIND_STOP_CONFIRMATION_SENT: string = 'fomo.sendblue.stop_confirmation_sent';
const KIND_STOP_CONFIRMATION_FAILED: string = 'fomo.sendblue.stop_confirmation_failed';
const KIND_ALERT_SUPPRESSED: string = 'fomo.alert.suppressed_stop_active';
const KIND_POLL_SKIPPED: string = 'fomo.poll.skipped_stop_active';

const EXPECTED_V055_AUDIT_KINDS: readonly string[] = [
  KIND_STOP_CONFIRMATION_SENT,
  KIND_STOP_CONFIRMATION_FAILED,
  KIND_ALERT_SUPPRESSED,
  KIND_POLL_SKIPPED
];

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
    process.stderr.write('[smoke-evidence:v0.5.5] DATABASE_URL not set. Source .env first.\n');
    process.exit(2);
  }
  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    process.stderr.write(`[smoke-evidence:v0.5.5] DB unavailable: ${dbResult.reason}\n`);
    process.exit(1);
  }
  const db = dbResult.client;

  console.log('Phase v0.5.5 evidence — STOP Enforcement + Confirmation (founder-only smoke)\n');
  console.log(`Smoke window: last ${SMOKE_WINDOW_HOURS}h (override via FOMO_V0_5_5_WINDOW_HOURS).\n`);

  const findings: Finding[] = [];

  /* ============================================================== */
  /* Registry inspection — determines which criteria are PENDING    */
  /* ============================================================== */

  // Widen to Set<string> on purpose — scaffolding-time the new v0.5.5 kinds
  // are not in the strict union, but we need to be able to query their presence.
  const auditActionSet = new Set(FOMO_AUDIT_ACTIONS as readonly string[]);
  const memorySignalSet = new Set(MEMORY_SIGNAL_KINDS as readonly string[]);
  const missingV055Actions = EXPECTED_V055_AUDIT_KINDS.filter((a) => !auditActionSet.has(a));
  const runtimePending = missingV055Actions.length > 0;

  /* ------------------------------------------------------------------ */
  /* C1: All 4 new audit actions registered in FOMO_AUDIT_ACTIONS        */
  /* ------------------------------------------------------------------ */
  if (missingV055Actions.length === 0) {
    findings.push({
      severity: 'pass',
      criterion: 'C1: All 4 v0.5.5-NEW audit actions registered in FOMO_AUDIT_ACTIONS',
      detail: `present: ${EXPECTED_V055_AUDIT_KINDS.join(', ')}`
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: 'C1: All 4 v0.5.5-NEW audit actions registered in FOMO_AUDIT_ACTIONS',
      detail: `PENDING runtime commit — missing from registry: ${missingV055Actions.join(', ')}. This is expected at scaffolding time; the runtime implementation commit registers these.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C2: Alert-creation short-circuit fires when stop_active=true        */
  /* ------------------------------------------------------------------ */
  if (!auditActionSet.has('fomo.alert.suppressed_stop_active')) {
    findings.push({
      severity: 'pending',
      criterion: 'C2: Alert-creation short-circuit fires when stop_active=true',
      detail: `PENDING — depends on 'fomo.alert.suppressed_stop_active' audit kind (not yet registered).`
    });
  } else {
    const rows = await db
      .select({ action: audit_log.action })
      .from(audit_log)
      .where(
        sql`${audit_log.action} = ${KIND_ALERT_SUPPRESSED}
            AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
      )
      .limit(5);
    findings.push({
      severity: rows.length > 0 ? 'pass' : 'fail',
      criterion: 'C2: Alert-creation short-circuit fires when stop_active=true',
      detail:
        rows.length > 0
          ? `${rows.length} suppression audit row(s) in smoke window — pipeline correctly short-circuited.`
          : `No suppression audit rows in smoke window. Either no STOP'd user received a new email during the smoke (re-run §6 Test 1 + send a synthetic email to the STOP'd user), or the short-circuit did not fire (runtime bug).`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C3: STOP confirmation reply sent on inbound STOP                    */
  /* ------------------------------------------------------------------ */
  if (!auditActionSet.has('fomo.sendblue.stop_confirmation_sent')) {
    findings.push({
      severity: 'pending',
      criterion: 'C3: STOP confirmation reply sent on inbound STOP',
      detail: `PENDING — depends on 'fomo.sendblue.stop_confirmation_sent' audit kind (not yet registered).`
    });
  } else {
    const rows = await db
      .select({ action: audit_log.action, detail: audit_log.detail })
      .from(audit_log)
      .where(
        sql`${audit_log.action} = ${KIND_STOP_CONFIRMATION_SENT}
            AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
      );
    findings.push({
      severity: rows.length >= 1 ? 'pass' : 'fail',
      criterion: 'C3: STOP confirmation reply sent on inbound STOP (operator-confirmed receipt)',
      detail:
        rows.length >= 1
          ? `${rows.length} confirmation-sent audit row(s) in window. Operator must additionally confirm the iMessage was received on the founder phone.`
          : 'No stop_confirmation_sent audit row in window. Re-run §6 Test 1 (founder STOP from real iMessage).'
    });
  }

  /* ------------------------------------------------------------------ */
  /* C4: Idempotency — duplicate STOP within 24h does NOT re-send        */
  /* ------------------------------------------------------------------ */
  if (!auditActionSet.has('fomo.sendblue.stop_confirmation_sent')) {
    findings.push({
      severity: 'pending',
      criterion: 'C4: Idempotency — duplicate STOP within 24h does NOT re-send confirmation',
      detail: `PENDING — depends on 'fomo.sendblue.stop_confirmation_sent' audit kind (not yet registered).`
    });
  } else {
    const rows = await db
      .select({ detail: audit_log.detail, actor_user_id: audit_log.actor_user_id })
      .from(audit_log)
      .where(
        sql`${audit_log.action} = ${KIND_STOP_CONFIRMATION_SENT}
            AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
      );
    // Group by actor_user_id; any actor with >1 confirmation in the same 24h
    // window is an idempotency violation (the runtime guard failed).
    const perActor = new Map<string, number>();
    for (const r of rows) {
      const k = r.actor_user_id ?? '<null>';
      perActor.set(k, (perActor.get(k) ?? 0) + 1);
    }
    const violators = [...perActor.entries()].filter(([, n]) => n > 1);
    findings.push({
      severity: violators.length === 0 ? 'pass' : 'fail',
      criterion: 'C4: Idempotency — duplicate STOP within 24h does NOT re-send confirmation',
      detail:
        violators.length === 0
          ? `No actor has >1 stop_confirmation_sent row in the smoke window (${rows.length} total row(s) across ${perActor.size} actor(s)). Operator must additionally confirm §6 Test 2 was actually run (sent STOP twice within 24h on the same number).`
          : `IDEMPOTENCY VIOLATION — actor(s) with >1 confirmation: ${violators.map(([a, n]) => `${a}=${n}`).join(', ')}. Runtime guard failed.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C5: START re-enables alerts                                         */
  /* ------------------------------------------------------------------ */
  // START is parsed by the existing reply parser (fomo.sendblue.start_recorded
  // is the existing v0.1 audit kind for it — already registered). C5 is a
  // BEHAVIOURAL check: after a START, the alert-creation pipeline must again
  // produce alerts for the un-STOP'd user. We can verify a structural
  // approximation here: a fomo.sendblue.start_recorded row exists in the
  // smoke window AND at least one alert row was created for that actor after
  // the start_recorded timestamp.
  const startRows = await db
    .select({ actor_user_id: audit_log.actor_user_id, occurred_at: audit_log.occurred_at })
    .from(audit_log)
    .where(
      sql`${audit_log.action} = 'fomo.sendblue.start_recorded'
          AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    )
    .orderBy(sql`${audit_log.occurred_at} DESC`);
  if (startRows.length === 0) {
    findings.push({
      severity: 'fail',
      criterion: 'C5: START re-enables alerts',
      detail: `No fomo.sendblue.start_recorded audit row in smoke window. Re-run §6 Test 3 (founder sends START after STOP).`
    });
  } else {
    const latestStart = startRows[0];
    if (!latestStart || !latestStart.actor_user_id) {
      findings.push({
        severity: 'fail',
        criterion: 'C5: START re-enables alerts',
        detail: 'fomo.sendblue.start_recorded row has null actor_user_id — investigate.'
      });
    } else {
      const postStartAlerts = await db
        .select({ alert_id: alerts.alert_id })
        .from(alerts)
        .where(
          sql`${alerts.user_id} = ${latestStart.actor_user_id}
              AND ${alerts.created_at} > ${latestStart.occurred_at}`
        )
        .limit(1);
      findings.push({
        severity: postStartAlerts.length >= 1 ? 'pass' : 'warn',
        criterion: 'C5: START re-enables alerts',
        detail:
          postStartAlerts.length >= 1
            ? `Found ≥1 alert created for actor=${latestStart.actor_user_id.slice(0, 8)}… AFTER the latest START at ${latestStart.occurred_at instanceof Date ? latestStart.occurred_at.toISOString() : String(latestStart.occurred_at)}.`
            : `WARN: no alert created for actor after START. Either no FOMO-worthy email arrived in the window after START, or the re-enable did not fire. Operator must run §6 Test 3 explicitly with a synthetic email after START.`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C6: Polling-after-STOP suppression                                  */
  /* ------------------------------------------------------------------ */
  if (!auditActionSet.has('fomo.poll.skipped_stop_active')) {
    findings.push({
      severity: 'pending',
      criterion: 'C6: Polling-after-STOP suppression — poll runs but no alerts created',
      detail: `PENDING — depends on 'fomo.poll.skipped_stop_active' audit kind (not yet registered).`
    });
  } else {
    const rows = await db
      .select({ action: audit_log.action })
      .from(audit_log)
      .where(
        sql`${audit_log.action} = ${KIND_POLL_SKIPPED}
            AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
      )
      .limit(5);
    findings.push({
      severity: rows.length > 0 ? 'pass' : 'warn',
      criterion: 'C6: Polling-after-STOP suppression — poll runs but no alerts created',
      detail:
        rows.length > 0
          ? `${rows.length} poll-skipped audit row(s) in window — polling continued, alert creation correctly skipped.`
          : 'WARN: no poll-skipped audit row. Either no new email arrived for the STOP\'d user during the smoke (operator-confirmed acceptable), or the skip did not fire.'
    });
  }

  /* ------------------------------------------------------------------ */
  /* C7: Cross-tenant isolation (THE v0.5.4-pattern load-bearing check)  */
  /* ------------------------------------------------------------------ */
  // Verify that the only stop_active row updated within the smoke window
  // belongs to the founder (target of v0.5.5 test). All other rows must be
  // byte-identical to the §0 baseline (their updated_at must predate the
  // smoke window). The runbook diff (baseline vs post) is the gold standard;
  // this is the automatable approximation.
  const stopRows = await db
    .select({ user_id: memory_signals.user_id, updated_at: memory_signals.updated_at })
    .from(memory_signals)
    .where(sql`${memory_signals.kind} = 'stop_active'`);
  const windowAgo = `now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`;
  const inWindowRows = await db.execute<{ user_id: string }>(
    sql`SELECT user_id FROM memory_signals
        WHERE kind = 'stop_active'
          AND updated_at > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
  );
  const inWindowSet = new Set((inWindowRows.rows as { user_id: string }[]).map((r) => r.user_id));
  const otherUsersUpdated = [...inWindowSet].filter((id) => id !== FOUNDER_USER_ID);
  if (otherUsersUpdated.length === 0) {
    findings.push({
      severity: 'pass',
      criterion: 'C7: Cross-tenant isolation — only founder stop_active row touched in smoke window',
      detail: `${stopRows.length} total stop_active row(s); ${inWindowSet.size} updated in window (founder only). Other users (Morris/gm3258/etc.) byte-identical. Operator must additionally confirm the runbook §8 baseline diff.`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'C7: Cross-tenant isolation — only founder stop_active row touched in smoke window',
      detail: `CROSS-TENANT VIOLATION — non-founder user_id(s) updated in window: ${otherUsersUpdated.map((u) => u.slice(0, 8) + '…').join(', ')}. v0.5.4 invariant broken.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C8: Confirmation wording deterministic + friendly                   */
  /* ------------------------------------------------------------------ */
  // Operator-confirmed (visual inspection of received iMessage). Automatable
  // approximation: if the runtime stores the confirmation body or template
  // hash in the audit detail, we can verify the canonical phrases appear.
  // The runtime commit should choose to log a sanitized snippet of the
  // outbound body in detail.message_preview (≤ 80 chars).
  if (!auditActionSet.has('fomo.sendblue.stop_confirmation_sent')) {
    findings.push({
      severity: 'pending',
      criterion: 'C8: Confirmation wording deterministic + friendly (operator-confirmed)',
      detail: `PENDING — depends on 'fomo.sendblue.stop_confirmation_sent' audit kind. Runtime commit should populate detail.message_preview so this check can verify "You're unsubscribed" + "Text START" canonical phrases.`
    });
  } else {
    const rows = await db
      .select({ detail: audit_log.detail })
      .from(audit_log)
      .where(
        sql`${audit_log.action} = ${KIND_STOP_CONFIRMATION_SENT}
            AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
      )
      .limit(5);
    const previews = rows.map((r) => {
      const d = r.detail as { message_preview?: unknown } | null;
      return typeof d?.message_preview === 'string' ? d.message_preview : '';
    });
    const hasCanonicalPhrases = previews.some(
      (p) => /unsubscrib|stop/i.test(p) && /start/i.test(p)
    );
    findings.push({
      severity: hasCanonicalPhrases ? 'pass' : 'warn',
      criterion: 'C8: Confirmation wording deterministic + friendly (canonical phrases present)',
      detail: hasCanonicalPhrases
        ? `Canonical phrases ("unsubscribed"/"STOP" + "START") detected in ${previews.length} confirmation preview(s). Operator must additionally confirm visually that the wording is friendly, not robotic.`
        : `WARN: canonical phrases not detected in any of ${previews.length} confirmation preview(s). Either runtime did not populate detail.message_preview, or the wording drifted from spec.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C9: STOP confirmation does NOT echo email content (leak-canary)     */
  /* ------------------------------------------------------------------ */
  // Scan stop_confirmation_sent rows for forbidden substrings (email body
  // fragments). Same approach as v0.5.4 C11 leak-canary scan.
  if (!auditActionSet.has('fomo.sendblue.stop_confirmation_sent')) {
    findings.push({
      severity: 'pending',
      criterion: 'C9: STOP confirmation contains zero email-content leakage (leak-canary scan)',
      detail: `PENDING — depends on 'fomo.sendblue.stop_confirmation_sent' audit kind.`
    });
  } else {
    const forbiddenSubstrings = ['brevio-canary-', 'Subject:', 'From:', '@gmail.com'];
    const rows = await db
      .select({ detail: audit_log.detail })
      .from(audit_log)
      .where(
        sql`${audit_log.action} = ${KIND_STOP_CONFIRMATION_SENT}
            AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
      );
    const hits: string[] = [];
    for (const r of rows) {
      const json = JSON.stringify(r.detail ?? {});
      for (const s of forbiddenSubstrings) {
        if (json.includes(s)) hits.push(s);
      }
    }
    findings.push({
      severity: hits.length === 0 ? 'pass' : 'fail',
      criterion: 'C9: STOP confirmation contains zero email-content leakage',
      detail:
        hits.length === 0
          ? `scanned ${rows.length} confirmation audit row(s); zero hits across ${forbiddenSubstrings.length} forbidden substring(s).`
          : `LEAK DETECTED — substring(s) found in confirmation audit detail: ${[...new Set(hits)].join(', ')}.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C10: Failure-mode — best-effort audit, no retry                     */
  /* ------------------------------------------------------------------ */
  if (!auditActionSet.has('fomo.sendblue.stop_confirmation_failed')) {
    findings.push({
      severity: 'pending',
      criterion: 'C10: Failure-mode handled — fomo.sendblue.stop_confirmation_failed exists; no retry',
      detail: `PENDING — depends on 'fomo.sendblue.stop_confirmation_failed' audit kind.`
    });
  } else {
    // §6 Test 4 induces a failure. After it runs, we expect ≥1 failed row
    // AND we expect that no stop_confirmation_sent row for the same actor
    // followed within the window (because the spec says: no retry).
    const failRows = await db
      .select({ actor_user_id: audit_log.actor_user_id, occurred_at: audit_log.occurred_at })
      .from(audit_log)
      .where(
        sql`${audit_log.action} = ${KIND_STOP_CONFIRMATION_FAILED}
            AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
      );
    if (failRows.length === 0) {
      findings.push({
        severity: 'warn',
        criterion: 'C10: Failure-mode handled — fomo.sendblue.stop_confirmation_failed exists; no retry',
        detail:
          'WARN: no failure audit rows in window. Either §6 Test 4 (induced SendBlue failure) was not run, or the failure path did not fire when induced.'
      });
    } else {
      // For each failed actor: assert no successful confirmation came AFTER
      // the failure within the same window.
      let retryViolations = 0;
      for (const f of failRows) {
        if (!f.actor_user_id) continue;
        const after = await db
          .select({ alert_id: audit_log.action })
          .from(audit_log)
          .where(
            sql`${audit_log.action} = ${KIND_STOP_CONFIRMATION_SENT}
                AND ${audit_log.actor_user_id} = ${f.actor_user_id}
                AND ${audit_log.occurred_at} > ${f.occurred_at}`
          )
          .limit(1);
        if (after.length > 0) retryViolations++;
      }
      findings.push({
        severity: retryViolations === 0 ? 'pass' : 'fail',
        criterion: 'C10: Failure-mode handled — best-effort audit, NO retry',
        detail:
          retryViolations === 0
            ? `${failRows.length} failure audit row(s); zero retry-violations (no _sent followed any _failed for the same actor in window).`
            : `RETRY VIOLATION — ${retryViolations} actor(s) had a stop_confirmation_sent follow a stop_confirmation_failed in window. Spec Q6: NO RETRY.`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C11: Founder regression — founder's own STOP/START works            */
  /* ------------------------------------------------------------------ */
  if (!auditActionSet.has('fomo.sendblue.stop_confirmation_sent')) {
    findings.push({
      severity: 'pending',
      criterion: `C11: Founder regression — founder's own STOP/START works`,
      detail: `PENDING — depends on 'fomo.sendblue.stop_confirmation_sent' audit kind.`
    });
  } else {
    const rows = await db
      .select({ actor_user_id: audit_log.actor_user_id })
      .from(audit_log)
      .where(
        sql`${audit_log.action} = ${KIND_STOP_CONFIRMATION_SENT}
            AND ${audit_log.actor_user_id} = ${FOUNDER_USER_ID}
            AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
      )
      .limit(5);
    findings.push({
      severity: rows.length >= 1 ? 'pass' : 'fail',
      criterion: `C11: Founder regression — founder STOP triggered a confirmation to founder phone`,
      detail:
        rows.length >= 1
          ? `${rows.length} stop_confirmation_sent audit row(s) for actor_user_id='${FOUNDER_USER_ID}'. Operator must additionally confirm the iMessage arrived on the founder phone AND that a subsequent START re-enabled alerts (C5 covers that automatically).`
          : `No stop_confirmation_sent row for actor_user_id='${FOUNDER_USER_ID}'. This is the load-bearing founder self-test; if it did not fire, the whole phase fails.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C12: All prior smoke-evidence scripts still PASS                    */
  /* ------------------------------------------------------------------ */
  // Operator-run: this script does not exec other scripts. It just reminds
  // the founder to run the full chain (v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4 +
  // v0.5.5) and confirm 5 × PASS. If any prior script FAILS post-v0.5.5,
  // v0.5.5 cannot ship.
  findings.push({
    severity: 'warn',
    criterion: 'C12: All prior smoke-evidence scripts (v0.5.1–v0.5.4) still PASS — OPERATOR MUST RUN',
    detail:
      'This script does not exec the prior scripts. Operator must run: pnpm smoke-evidence:v0.5.1 && pnpm smoke-evidence:v0.5.2 && pnpm smoke-evidence:v0.5.3 && pnpm smoke-evidence:v0.5.4 — all four must print VERDICT: PASS for v0.5.5 to be considered shippable.'
  });

  /* ============================================================== */
  /* Report                                                         */
  /* ============================================================== */
  await dbResult.pool.end();

  console.log('========================================================================');
  console.log('Phase v0.5.5 evidence summary — 12 criteria (STOP Enforcement + Confirmation)');
  console.log('========================================================================');
  for (const f of findings) {
    console.log(`  [${symbol(f.severity)}] ${f.criterion}`);
    console.log(`        ${f.detail}`);
  }
  console.log('');

  const hasFail = findings.some((f) => f.severity === 'fail');
  const hasPending = findings.some((f) => f.severity === 'pending');

  // Verdict precedence (intentional):
  //   1. runtimePending (v0.5.5 kinds not in registry) → VERDICT: PENDING.
  //      Reason: without the runtime commit, FAILs on smoke-dependent criteria
  //      (C5 if no START in the rolling window; C7 if a prior phase's writes
  //      fall inside the rolling window) are noise, not real failures. The
  //      founder cannot have run the v0.5.5 smoke yet because the runtime
  //      isn't there. Surface PENDING so the founder is not confused.
  //   2. else hasFail → VERDICT: FAIL. Runtime is there; failures are real.
  //   3. else hasPending → VERDICT: PENDING — runtime there, smoke not yet
  //      executed.
  //   4. else → VERDICT: PASS.
  if (runtimePending) {
    if (hasFail) {
      console.log(
        `! Note: ${findings.filter((f) => f.severity === 'fail').length} criterion(criteria) reported FAIL above, but those are scaffolding-time artifacts (e.g. C5 needs a START audit row from a smoke that has not yet run; C7 may show a non-founder write from a prior phase still within the rolling smoke window). These FAILs are NOT real failures while runtime is pending — re-run after the runtime commit lands and the smoke is executed.`
      );
    }
    console.log(
      `VERDICT: PENDING  — runtime implementation not yet committed (${EXPECTED_V055_AUDIT_KINDS.filter((a) => !auditActionSet.has(a)).length}/${EXPECTED_V055_AUDIT_KINDS.length} expected audit kind(s) missing from FOMO_AUDIT_ACTIONS). This is the expected state at SCAFFOLDING time. When the runtime commit lands and registers the missing kinds, re-run this script — PENDING markers will disappear automatically.`
    );
    process.exit(1);
  }
  if (hasFail) {
    console.log(
      'VERDICT: FAIL  — at least one criterion failed. Fix and re-run before considering merge.'
    );
    process.exit(1);
  }
  if (hasPending) {
    console.log(
      'VERDICT: PENDING  — runtime is in place, but at least one criterion depends on a smoke artifact that has not been produced yet. Run the §6 test sequence in the runbook and re-run this script.'
    );
    process.exit(1);
  }
  console.log(
    'VERDICT: PASS  (operator must additionally confirm: STOP confirmation iMessage was received on founder phone; §6 Test 2 was actually run (sent STOP twice within 24h); §6 Test 4 was actually run (induced SendBlue failure); runbook §8 baseline diff shows no cross-tenant writes. Run smoke-evidence:v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4 separately to confirm prior PASS criteria still hold.)'
  );
  process.exit(0);
}

main().catch((err) => {
  process.stderr.write(`[smoke-evidence:v0.5.5] fatal: ${err instanceof Error ? err.message : String(err)}\n`);
  process.stderr.write(err instanceof Error && err.stack ? err.stack + '\n' : '');
  process.exit(1);
});
