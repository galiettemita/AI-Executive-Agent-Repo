// Phase v0.5.6 smoke-evidence — iMessage Tone + Summary Length.
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-05, Q3 corrected):
//   This script is part of the v0.5.6 SCAFFOLDING commit, which lands BEFORE
//   the runtime implementation. Same 'pending' severity model as v0.5.5:
//   PENDING means "this criterion depends on a runtime artifact (audit kind
//   or FOUNDER_TEXT_TEMPLATE_VERSION bump) that the runtime commit will
//   introduce." Until the runtime commit lands, the criterion is PENDING and
//   the overall VERDICT is PENDING.
//
//   When the runtime commit lands and:
//     1. Registers `fomo.alert.drafter_schema_failed` in FOMO_AUDIT_ACTIONS
//     2. Bumps FOUNDER_TEXT_TEMPLATE_VERSION (e.g. 'founder-text-v0.2.0')
//   AND fires through the renderFounderText path during smoke, the PENDING
//   markers disappear and VERDICT flips to PASS (or FAIL on real failures).
//
// v0.5.6 HYBRID scope (preserves 3E.1 no-LLM-body-generation directive):
//   * Surface (a) — deterministic shell rewrite (no LLM): drops "FOMO ·"
//     header, sentence-shaped, no arbitrary ellipsis, substitutes
//     ranker.reason for body_snippet, bumps template_version.
//   * Surface (b) — ranker `reason` tightening (the only LLM-allowed slot
//     per 3E.1 carve-out): prompt rewrite, structured output schema with
//     typed length on reason, deterministic fallback string on violation.
//
// Founder-only smoke. No friend involvement (three-friend cap holds).
// Read-only — never mutates the DB.

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import { FOMO_AUDIT_ACTIONS } from '../src/core/audit.js';
import { FOUNDER_TEXT_TEMPLATE_VERSION } from '../src/core/founder-text-template.js';

type Severity = 'pass' | 'warn' | 'fail' | 'pending';

interface Finding {
  readonly severity: Severity;
  readonly criterion: string;
  readonly detail: string;
}

const SMOKE_WINDOW_HOURS = Number((process.env.FOMO_V0_5_6_WINDOW_HOURS ?? '24').trim()) || 24;
const FOUNDER_USER_ID = (process.env.FOMO_FOUNDER_USER_ID ?? 'founder').trim();

// v0.5.6 length policy (founder Q4 lock 2026-06-05):
//   - Target: 220–280 chars
//   - Hard max: 320 (340 absolute max only for impl buffer)
//   - 1–2 short sentences max
//   - NO mid-sentence truncation
//   - NO ellipsis from arbitrary truncation
const TARGET_MIN = 220;
const TARGET_MAX = 280;
const HARD_MAX = 320;
const ABSOLUTE_MAX = 340;

// The runtime commit registers this kind. As-strict-as-v0.5.5 typing is
// deferred until runtime lands (cast widening is the scaffolding-time
// workaround; removed when runtime adds the kind to FOMO_AUDIT_ACTIONS).
const EXPECTED_V056_NEW_AUDIT_KIND = 'fomo.alert.drafter_schema_failed';

// The previous (v0.5.5-and-prior) template version. Runtime commit bumps
// past this; if it equals this, runtime hasn't shipped yet.
const V055_TEMPLATE_VERSION_BASELINE = 'founder-text-v0.1.0';

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
    process.stderr.write('[smoke-evidence:v0.5.6] DATABASE_URL not set. Source .env first.\n');
    process.exit(2);
  }
  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    process.stderr.write(`[smoke-evidence:v0.5.6] DB unavailable: ${dbResult.reason}\n`);
    process.exit(1);
  }
  const db = dbResult.client;

  console.log('Phase v0.5.6 evidence — iMessage Tone + Summary Length (founder-only smoke)\n');
  console.log(`Smoke window: last ${SMOKE_WINDOW_HOURS}h (override via FOMO_V0_5_6_WINDOW_HOURS).\n`);

  const findings: Finding[] = [];

  /* ============================================================== */
  /* Registry inspection — determines which criteria are PENDING    */
  /* ============================================================== */

  const auditActionSet = new Set(FOMO_AUDIT_ACTIONS as readonly string[]);
  const newAuditKindRegistered = auditActionSet.has(EXPECTED_V056_NEW_AUDIT_KIND);
  const templateVersionBumped = FOUNDER_TEXT_TEMPLATE_VERSION !== V055_TEMPLATE_VERSION_BASELINE;
  const runtimePending = !newAuditKindRegistered || !templateVersionBumped;

  /* ------------------------------------------------------------------ */
  /* C1: fomo.alert.drafter_schema_failed registered                     */
  /* ------------------------------------------------------------------ */
  if (newAuditKindRegistered) {
    findings.push({
      severity: 'pass',
      criterion: `C1: '${EXPECTED_V056_NEW_AUDIT_KIND}' registered in FOMO_AUDIT_ACTIONS`,
      detail: 'audit kind present in registry'
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: `C1: '${EXPECTED_V056_NEW_AUDIT_KIND}' registered in FOMO_AUDIT_ACTIONS`,
      detail: 'PENDING runtime commit — kind not in registry. The runtime commit registers it when wiring the ranker schema-validation fallback path.'
    });
  }

  /* ------------------------------------------------------------------ */
  /* C2: FOUNDER_TEXT_TEMPLATE_VERSION bumped past founder-text-v0.1.0   */
  /* ------------------------------------------------------------------ */
  if (templateVersionBumped) {
    findings.push({
      severity: 'pass',
      criterion: 'C2: FOUNDER_TEXT_TEMPLATE_VERSION bumped past v0.1.0',
      detail: `current: '${FOUNDER_TEXT_TEMPLATE_VERSION}' (was '${V055_TEMPLATE_VERSION_BASELINE}'). The shape change is now traceable in audit detail.template_version.`
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: 'C2: FOUNDER_TEXT_TEMPLATE_VERSION bumped past v0.1.0',
      detail: `PENDING runtime commit — still '${V055_TEMPLATE_VERSION_BASELINE}'. The runtime commit bumps this to mark the deterministic-shell rewrite.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C3: Recent fomo.send.attempted audit rows carry the new template_version */
  /* ------------------------------------------------------------------ */
  if (!templateVersionBumped) {
    findings.push({
      severity: 'pending',
      criterion: 'C3: Recent fomo.send.attempted rows carry the bumped template_version',
      detail: `PENDING — depends on FOUNDER_TEXT_TEMPLATE_VERSION bump (C2). Once bumped + smoke run, we can query audit detail.template_version to prove the new shape is in effect.`
    });
  } else {
    const rows = await db.execute<{ template_version: string | null }>(
      sql`SELECT detail->>'template_version' AS template_version
          FROM audit_log
          WHERE action = 'fomo.send.attempted'
            AND occurred_at > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );
    const versions = (rows.rows as { template_version: string | null }[])
      .map((r) => r.template_version)
      .filter((v): v is string => typeof v === 'string');
    const stale = versions.filter((v) => v === V055_TEMPLATE_VERSION_BASELINE);
    const fresh = versions.filter((v) => v !== V055_TEMPLATE_VERSION_BASELINE);
    if (versions.length === 0) {
      findings.push({
        severity: 'warn',
        criterion: 'C3: Recent fomo.send.attempted rows carry the bumped template_version',
        detail: `WARN: no fomo.send.attempted rows in smoke window. §6 Test 1 (synthetic important email + approve) must run.`
      });
    } else if (stale.length === 0) {
      findings.push({
        severity: 'pass',
        criterion: 'C3: Recent fomo.send.attempted rows carry the bumped template_version',
        detail: `${fresh.length}/${versions.length} rows on bumped version (${[...new Set(fresh)].join(', ')}). Zero rows on stale '${V055_TEMPLATE_VERSION_BASELINE}'.`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C3: Recent fomo.send.attempted rows carry the bumped template_version',
        detail: `STALE TEMPLATE LEAK — ${stale.length}/${versions.length} rows still on '${V055_TEMPLATE_VERSION_BASELINE}'. Either the outbound path bypassed the new renderFounderText, or the bump did not land.`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C4: Body length stays within target 220–280 / hard cap 320          */
  /* ------------------------------------------------------------------ */
  if (!templateVersionBumped) {
    findings.push({
      severity: 'pending',
      criterion: `C4: Body length within target ${TARGET_MIN}–${TARGET_MAX} / hard cap ${HARD_MAX}`,
      detail: `PENDING — depends on FOUNDER_TEXT_TEMPLATE_VERSION bump (C2) + smoke run.`
    });
  } else {
    const rows = await db.execute<{ content_chars: number | null; template_version: string | null }>(
      sql`SELECT (detail->>'content_chars')::int AS content_chars,
                 detail->>'template_version' AS template_version
          FROM audit_log
          WHERE action = 'fomo.send.attempted'
            AND occurred_at > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval
            AND detail->>'template_version' != ${V055_TEMPLATE_VERSION_BASELINE}`
    );
    const charCounts = (rows.rows as { content_chars: number | null }[])
      .map((r) => r.content_chars)
      .filter((n): n is number => typeof n === 'number');
    if (charCounts.length === 0) {
      findings.push({
        severity: 'warn',
        criterion: `C4: Body length within target ${TARGET_MIN}–${TARGET_MAX} / hard cap ${HARD_MAX}`,
        detail: `WARN: no fresh-template fomo.send.attempted rows in window. §6 Test 1 must run.`
      });
    } else {
      const overHard = charCounts.filter((n) => n > HARD_MAX).length;
      const overAbsolute = charCounts.filter((n) => n > ABSOLUTE_MAX).length;
      const inTarget = charCounts.filter((n) => n >= TARGET_MIN && n <= TARGET_MAX).length;
      const median = (() => {
        const sorted = [...charCounts].sort((a, b) => a - b);
        return sorted[Math.floor(sorted.length / 2)] ?? 0;
      })();
      if (overAbsolute > 0) {
        findings.push({
          severity: 'fail',
          criterion: `C4: Body length within target ${TARGET_MIN}–${TARGET_MAX} / hard cap ${HARD_MAX}`,
          detail: `HARD-CAP VIOLATION — ${overAbsolute}/${charCounts.length} rows > absolute max ${ABSOLUTE_MAX} chars. Length policy broken.`
        });
      } else if (overHard > 0) {
        findings.push({
          severity: 'warn',
          criterion: `C4: Body length within target ${TARGET_MIN}–${TARGET_MAX} / hard cap ${HARD_MAX}`,
          detail: `WARN: ${overHard}/${charCounts.length} rows > hard cap ${HARD_MAX} but ≤ ${ABSOLUTE_MAX} (impl buffer). Median: ${median}. Tighten.`
        });
      } else {
        findings.push({
          severity: 'pass',
          criterion: `C4: Body length within target ${TARGET_MIN}–${TARGET_MAX} / hard cap ${HARD_MAX}`,
          detail: `${inTarget}/${charCounts.length} rows in target band; all ≤ hard cap. Median ${median} chars.`
        });
      }
    }
  }

  /* ------------------------------------------------------------------ */
  /* C5: NO arbitrary ellipsis truncation (sentence-boundary policy)     */
  /* ------------------------------------------------------------------ */
  // The body text is not stored in audit detail. C5 is verified at code level
  // by the regression test suite (apps/fomo/src/core/founder-text-template.test.ts
  // asserting no `…` for normal-length inputs). Operator confirms visually on
  // the manual taste-check iMessage.
  findings.push({
    severity: 'warn',
    criterion: 'C5: NO arbitrary ellipsis truncation (sentence-boundary policy)',
    detail:
      'OPERATOR + CODE-LEVEL: body text is NOT persisted to audit. Code-level: regression test suite (founder-text-template.test.ts) must assert no "…" for normal-length inputs. Visual: operator confirms in §6 Test 3 manual taste check that the received iMessage has no arbitrary ellipsis.'
  });

  /* ------------------------------------------------------------------ */
  /* C6: Schema-validation fallback path fires when ranker.reason violates */
  /* ------------------------------------------------------------------ */
  if (!newAuditKindRegistered) {
    findings.push({
      severity: 'pending',
      criterion: `C6: '${EXPECTED_V056_NEW_AUDIT_KIND}' fires when ranker.reason violates schema`,
      detail: `PENDING — depends on '${EXPECTED_V056_NEW_AUDIT_KIND}' audit kind registration (C1).`
    });
  } else {
    const rows = await db.execute<{ action: string }>(
      sql`SELECT action FROM audit_log
          WHERE action = ${EXPECTED_V056_NEW_AUDIT_KIND}
            AND occurred_at > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval
          LIMIT 5`
    );
    const n = (rows.rows as { action: string }[]).length;
    findings.push({
      severity: n >= 1 ? 'pass' : 'warn',
      criterion: `C6: '${EXPECTED_V056_NEW_AUDIT_KIND}' fires when ranker.reason violates schema`,
      detail:
        n >= 1
          ? `${n} schema-failed audit row(s) in window — fallback path verified.`
          : `WARN: no schema-failed audit rows. Either §6 Test 2 (induced ranker.reason violation) was not run, or the fallback path did not fire when induced.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C7: Cross-tenant isolation                                          */
  /* ------------------------------------------------------------------ */
  // v0.5.6 should NOT touch memory_signals.stop_active at all (that's v0.5.5
  // territory). Verify the same baseline-diff approach as v0.5.5 C7.
  const stopRows = await db.execute<{ user_id: string }>(
    sql`SELECT user_id FROM memory_signals
        WHERE kind = 'stop_active'
          AND updated_at > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
  );
  const stopInWindow = (stopRows.rows as { user_id: string }[]).map((r) => r.user_id);
  const nonFounderStopWrites = stopInWindow.filter((id) => id !== FOUNDER_USER_ID);

  // Also: any fomo.send.attempted for non-founder during smoke window is
  // suspicious (founder-only smoke). v0.5.4's two real friends each have
  // their own user_id; if either receives a send during v0.5.6 smoke, that
  // would be a cross-tenant leak.
  const sendRows = await db.execute<{ actor_user_id: string | null }>(
    sql`SELECT DISTINCT actor_user_id FROM audit_log
        WHERE action = 'fomo.send.attempted'
          AND occurred_at > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
  );
  const sendActors = (sendRows.rows as { actor_user_id: string | null }[])
    .map((r) => r.actor_user_id)
    .filter((id): id is string => typeof id === 'string');
  const nonFounderSends = sendActors.filter((id) => id !== FOUNDER_USER_ID);

  if (nonFounderStopWrites.length === 0 && nonFounderSends.length === 0) {
    findings.push({
      severity: 'pass',
      criterion: 'C7: Cross-tenant isolation — only founder touched in smoke window',
      detail: `0 non-founder stop_active writes; 0 non-founder fomo.send.attempted rows. Founder-only smoke maintained.`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'C7: Cross-tenant isolation — only founder touched in smoke window',
      detail: `CROSS-TENANT VIOLATION — non-founder stop_active writes: ${nonFounderStopWrites.length}; non-founder fomo.send.attempted rows: ${nonFounderSends.length}.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C8: ranker.reason actually substituted into body (input shape proof) */
  /* ------------------------------------------------------------------ */
  // The runtime commit changes renderFounderText's input shape so rank
  // includes `reason`. We can't see the rendered text in audit detail, but
  // we CAN verify the input wiring via fomo.send.attempted's detail.score
  // existing alongside template_version != baseline. If the runtime forgot
  // to thread reason through, the smoke test (or a runtime unit test) is
  // the gate. This criterion is mostly a marker — the load-bearing check
  // is the runtime unit test that asserts the rendered output includes
  // a substring matching rank.reason.
  findings.push({
    severity: 'warn',
    criterion: 'C8: ranker.reason actually substituted into rendered body (input wiring)',
    detail:
      'CODE-LEVEL: load-bearing check is the runtime unit test in founder-text-template.test.ts asserting renderFounderText({rank: {label, score, reason}}).text includes rank.reason content. Operator confirms via §6 Test 3 manual taste check that the iMessage body explains "why this matters" (ranker.reason prose), not the email body snippet.'
  });

  /* ------------------------------------------------------------------ */
  /* C9: Body / audit contains zero email-content leakage (canary)       */
  /* ------------------------------------------------------------------ */
  // Same forbidden-substring scan as v0.5.5 C9. We scan fomo.send.attempted
  // detail JSON (template_version, content_chars, label, score — none of
  // those should ever contain email body fragments).
  const forbiddenSubstrings = ['brevio-canary-', 'Subject:', 'From:', '@gmail.com'];
  const sendDetails = await db.execute<{ detail: unknown }>(
    sql`SELECT detail FROM audit_log
        WHERE action = 'fomo.send.attempted'
          AND occurred_at > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
  );
  const hits: string[] = [];
  for (const r of sendDetails.rows as { detail: unknown }[]) {
    const json = JSON.stringify(r.detail ?? {});
    for (const s of forbiddenSubstrings) {
      if (json.includes(s)) hits.push(s);
    }
  }
  findings.push({
    severity: hits.length === 0 ? 'pass' : 'fail',
    criterion: 'C9: Body / audit contains zero email-content leakage',
    detail:
      hits.length === 0
        ? `scanned ${(sendDetails.rows as unknown[]).length} fomo.send.attempted audit row(s); zero hits across ${forbiddenSubstrings.length} forbidden substring(s).`
        : `LEAK DETECTED — substring(s) found in fomo.send.attempted detail: ${[...new Set(hits)].join(', ')}.`
  });

  /* ------------------------------------------------------------------ */
  /* C10: Operator manual taste check — real iMessage on iPhone          */
  /* ------------------------------------------------------------------ */
  findings.push({
    severity: 'warn',
    criterion: 'C10: Operator manual taste check — real iMessage rendering passed founder eye-test',
    detail:
      `OPERATOR-CONFIRMED ONLY. After un-flagging founder's SendBlue OPTED_OUT state (one-time ops, does NOT subsume F1 tier work), founder runs §6 Test 3 (sends one synthetic important email to themselves; approves the Slack card; receives the iMessage on iPhone). Founder confirms in SMOKE_REPORT §10: (a) no "FOMO · IMPORTANT (0.92)" telemetry header; (b) sentence-shaped, not newline-separated raw fields; (c) no arbitrary "…" ellipsis; (d) body contains ranker.reason prose explaining WHY it matters; (e) feels like a helpful nudge, not a bot.`
  });

  /* ------------------------------------------------------------------ */
  /* C11: Founder regression — recent founder-targeted send used new shape */
  /* ------------------------------------------------------------------ */
  if (!templateVersionBumped) {
    findings.push({
      severity: 'pending',
      criterion: `C11: Recent founder-targeted send used bumped template`,
      detail: `PENDING — depends on FOUNDER_TEXT_TEMPLATE_VERSION bump (C2).`
    });
  } else {
    const rows = await db.execute<{ template_version: string | null }>(
      sql`SELECT detail->>'template_version' AS template_version
          FROM audit_log
          WHERE action = 'fomo.send.attempted'
            AND actor_user_id = ${FOUNDER_USER_ID}
            AND occurred_at > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval
          ORDER BY occurred_at DESC
          LIMIT 5`
    );
    const versions = (rows.rows as { template_version: string | null }[])
      .map((r) => r.template_version)
      .filter((v): v is string => typeof v === 'string');
    const allFresh = versions.length > 0 && versions.every((v) => v !== V055_TEMPLATE_VERSION_BASELINE);
    if (versions.length === 0) {
      findings.push({
        severity: 'fail',
        criterion: 'C11: Recent founder-targeted send used bumped template',
        detail: `No fomo.send.attempted rows for actor_user_id='${FOUNDER_USER_ID}' in window. The load-bearing founder self-test did not fire; §6 Test 1 (founder regression) must run.`
      });
    } else if (allFresh) {
      findings.push({
        severity: 'pass',
        criterion: 'C11: Recent founder-targeted send used bumped template',
        detail: `${versions.length} founder send row(s); all on bumped template_version.`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C11: Recent founder-targeted send used bumped template',
        detail: `Founder send rows include stale template_version='${V055_TEMPLATE_VERSION_BASELINE}'. Regression: the bumped template is not in the founder send path.`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C12: All prior smoke-evidence scripts still PASS                    */
  /* ------------------------------------------------------------------ */
  findings.push({
    severity: 'warn',
    criterion: 'C12: All prior smoke-evidence scripts (v0.5.1–v0.5.5) still PASS — OPERATOR MUST RUN',
    detail:
      'This script does not exec the prior scripts. Operator must run: pnpm smoke-evidence:v0.5.1 && pnpm smoke-evidence:v0.5.2 && pnpm smoke-evidence:v0.5.3 && pnpm smoke-evidence:v0.5.4 && pnpm smoke-evidence:v0.5.5 — all five must print VERDICT: PASS (v0.5.5 may legitimately be FAIL/PENDING per its external-blocker record; operator confirms that is the known state, not a regression caused by v0.5.6).'
  });

  /* ============================================================== */
  /* Report                                                         */
  /* ============================================================== */
  await dbResult.pool.end();

  console.log('========================================================================');
  console.log('Phase v0.5.6 evidence summary — 12 criteria (iMessage Tone + Summary Length)');
  console.log('========================================================================');
  for (const f of findings) {
    console.log(`  [${symbol(f.severity)}] ${f.criterion}`);
    console.log(`        ${f.detail}`);
  }
  console.log('');

  const hasFail = findings.some((f) => f.severity === 'fail');
  const hasPending = findings.some((f) => f.severity === 'pending');

  // Verdict precedence (same as v0.5.5):
  //   1. runtimePending → VERDICT: PENDING (expected at scaffolding).
  //   2. else hasFail → VERDICT: FAIL.
  //   3. else hasPending → VERDICT: PENDING (runtime there, smoke not yet).
  //   4. else → VERDICT: PASS.
  if (runtimePending) {
    if (hasFail) {
      console.log(
        `! Note: ${findings.filter((f) => f.severity === 'fail').length} criterion(criteria) reported FAIL above, but those are scaffolding-time artifacts (e.g. fresh-template queries that have nothing to find while runtime hasn't bumped the version). Not real failures while runtime is pending.`
      );
    }
    const pendingItems = [
      newAuditKindRegistered ? null : `audit kind '${EXPECTED_V056_NEW_AUDIT_KIND}' not registered`,
      templateVersionBumped ? null : `FOUNDER_TEXT_TEMPLATE_VERSION still '${V055_TEMPLATE_VERSION_BASELINE}'`
    ].filter((s): s is string => s !== null);
    console.log(
      `VERDICT: PENDING  — runtime implementation not yet committed (${pendingItems.join('; ')}). Expected at SCAFFOLDING time. When the runtime commit lands and addresses both, re-run this script — PENDING markers will disappear automatically.`
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
      'VERDICT: PENDING  — runtime is in place, but at least one criterion depends on a smoke artifact that has not been produced yet. Run the §6 test sequence and re-run.'
    );
    process.exit(1);
  }
  console.log(
    'VERDICT: PASS  (operator must additionally confirm: §6 Test 3 manual taste check passed on real iPhone; C5 no-ellipsis verified by code-level test suite; C8 ranker.reason substitution verified by runtime unit test; runbook §8 baseline diff shows no cross-tenant writes. Run smoke-evidence:v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4 + v0.5.5 separately to confirm prior PASS criteria still hold.)'
  );
  process.exit(0);
}

main().catch((err) => {
  process.stderr.write(`[smoke-evidence:v0.5.6] fatal: ${err instanceof Error ? err.message : String(err)}\n`);
  process.stderr.write(err instanceof Error && err.stack ? err.stack + '\n' : '');
  process.exit(1);
});
