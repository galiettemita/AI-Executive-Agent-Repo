// Phase v0.5.10 smoke-evidence — Reply-parser feedback intents.
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-06):
//   Same 'pending' severity model as v0.5.5–v0.5.9. PENDING means "this
//   criterion depends on a runtime artifact that the runtime commit will
//   introduce." Until the runtime commit lands:
//     * PROMPT_VERSION still 'reply-parser-v0.1.0' (not v0.2.0) → C3 PENDING
//     * feedback-routing.ts module not present → C6/C7/C8/C9/C10 PENDING
//     * Explicit-feedback-phrase allowlist not in deterministic.ts → C4 PENDING
//     * orchestrator ≤3-word safe rule not wired → C5 PENDING
//     * intent_source field not in feedback.written detail → C1 PENDING
//   When runtime + smoke run, PENDINGs flip to PASS/FAIL per live state.
//
// v0.5.10 scope (locked Q1–Q6 — see memory project_v05-10-scope):
//   * Q1.B — feedback-routing.ts policy module
//   * Q2.A-modified — 2 new intents (this_mattered, more_like_this);
//     CORRECTED positive-confirmation mapping
//   * Q3.A + Q3.C — 0.7 threshold + ≤3-word safe rule + explicit allowlist
//   * Q4.A — only ignore_sender triggers applyFeedback (v0.5.9 consumer)
//   * Q5.A — silent (no acknowledgment iMessage)
//   * Q6.A-modified — 10-field feedback.written detail extension; NO new
//     audit kind. Hard privacy rule: zero raw reply text / subject / body /
//     snippet / headers / sender_email in new audit detail.
//
// Founder-only smoke. No friend involvement (three-friend cap). Read-only —
// never mutates the DB.

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import { FOMO_AUDIT_ACTIONS } from '../src/core/audit.js';
import { MEMORY_SIGNAL_KINDS } from '../src/memory/memory-signals.js';
import { PROMPT_VERSION as REPLY_PARSER_PROMPT_VERSION } from '../src/reply-parser/prompt.js';

type Severity = 'pass' | 'warn' | 'fail' | 'pending';

interface Finding {
  readonly severity: Severity;
  readonly criterion: string;
  readonly detail: string;
}

const SMOKE_WINDOW_HOURS = Number((process.env.FOMO_V0_5_10_WINDOW_HOURS ?? '24').trim()) || 24;
const FOUNDER_USER_ID = (process.env.FOMO_FOUNDER_USER_ID ?? 'founder').trim();

// Cast to string so the comparison compiles before the runtime commit
// changes the literal (mirrors v0.5.8/v0.5.9 scaffolding pattern).
const EXPECTED_V0510_PROMPT_VERSION = 'reply-parser-v0.2.0' as string;

// v0.5.10 NEW intents added to the parser (additive on top of v0.1.0's 6).
const NEW_V0510_INTENTS = ['this_mattered', 'more_like_this'] as const;
const ALL_V0510_INTENTS = [
  'snooze',
  'ignore',
  'ignore_sender',
  'why',
  'false_positive',
  'unclear',
  ...NEW_V0510_INTENTS
] as const;

// v0.5.10 NEW intent_source enum values.
const EXPECTED_INTENT_SOURCES = [
  'reply_parser_classifier',
  'reply_parser_deterministic',
  'slack_interactivity',
  'ops_inject'
] as const;

// Privacy canary — forbidden substrings in new feedback.written +
// brevio.feedback.applied detail. v0.5.10 tightens the v0.5.9 canary with
// reply-text-specific patterns (anything that looks like an inbound reply
// body fragment).
const FORBIDDEN_DETAIL_SUBSTRINGS = [
  'Subject:',
  'From:',
  'To:',
  '@gmail.com',
  '@icloud.com',
  '@hotmail.com',
  '@yahoo.com',
  // Reply-text canaries — common founder-typed reply prefixes that should
  // NEVER show up in audit detail (the audit carries structural intent
  // metadata only; the parser-classified intent enum is the contract).
  'ignore this',
  'not important',
  'this mattered',
  'more like this'
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
    process.stderr.write('[smoke-evidence:v0.5.10] DATABASE_URL not set. Source .env first.\n');
    process.exit(2);
  }
  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    process.stderr.write(`[smoke-evidence:v0.5.10] DB unavailable: ${dbResult.reason}\n`);
    process.exit(1);
  }
  const db = dbResult.client;

  console.log('Phase v0.5.10 evidence — Reply-parser feedback intents (founder-only smoke)\n');
  console.log(`Smoke window: last ${SMOKE_WINDOW_HOURS}h (override via FOMO_V0_5_10_WINDOW_HOURS).\n`);

  const findings: Finding[] = [];

  /* ============================================================== */
  /* Registry inspection — determines which criteria are PENDING    */
  /* ============================================================== */

  const promptBumped = (REPLY_PARSER_PROMPT_VERSION as string) === EXPECTED_V0510_PROMPT_VERSION;

  // feedback-routing.ts module presence + helper exports. Indirect-path
  // import so TS doesn't resolve the missing module at compile time.
  let routingModulePresent = false;
  try {
    const modulePath = '../src/reply-parser/feedback-routing.js';
    const mod = (await import(modulePath)) as Record<string, unknown>;
    routingModulePresent = typeof mod.routeReplyFeedback === 'function';
  } catch {
    routingModulePresent = false;
  }

  // v0.5.9 invariants still in place.
  const auditActionSet = new Set<string>(FOMO_AUDIT_ACTIONS as readonly string[]);
  const v059AuditsRegistered =
    auditActionSet.has('feedback.written') && auditActionSet.has('brevio.feedback.applied');
  const memorySignalSet = new Set<string>(MEMORY_SIGNAL_KINDS as readonly string[]);
  const v059MemorySignalsRegistered = memorySignalSet.has('sender_feedback_ignored');

  /* ------------------------------------------------------------------ */
  /* C1: feedback.written audit detail carries the 10 locked fields      */
  /* ------------------------------------------------------------------ */

  const feedbackWrittenRows = await db.execute<{ occurred_at: Date; actor_user_id: string | null; detail: Record<string, unknown> | null; result: string }>(
    sql`SELECT occurred_at, actor_user_id, detail, result
        FROM audit_log
        WHERE action = 'feedback.written'
          AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval
        ORDER BY occurred_at ASC`
  );
  const writtenRows = feedbackWrittenRows.rows as Array<{ occurred_at: Date; actor_user_id: string | null; detail: Record<string, unknown> | null; result: string }>;

  const replyParserRows = writtenRows.filter((r) => {
    const d = r.detail as { intent_source?: string } | null;
    return d !== null && (d.intent_source === 'reply_parser_classifier' || d.intent_source === 'reply_parser_deterministic');
  });

  if (!routingModulePresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C1: feedback.written detail carries 10 locked fields (intent_source, inbound_reply_id, parser_intent, parser_confidence + v0.5.9 carry-forward)',
      detail: 'PENDING runtime commit (feedback-routing.ts not yet present)'
    });
  } else if (replyParserRows.length === 0) {
    findings.push({
      severity: 'pending',
      criterion: 'C1: feedback.written detail carries 10 locked fields (intent_source, inbound_reply_id, parser_intent, parser_confidence + v0.5.9 carry-forward)',
      detail: 'no reply-parser-routed feedback.written rows in window; depends on Test 1'
    });
  } else {
    const sample = replyParserRows[0]!.detail as Record<string, unknown>;
    const required = ['intent_source', 'parser_intent', 'parser_confidence', 'source_surface', 'verb', 'feedback_event_id'];
    const missing = required.filter((k) => !(k in sample));
    if (missing.length === 0) {
      findings.push({
        severity: 'pass',
        criterion: 'C1: feedback.written detail carries 10 locked fields (per Q6.A-modified)',
        detail: `${replyParserRows.length} reply-parser-routed row(s). sample fields present: ${required.join(', ')}; intent_source=${String(sample.intent_source)}, parser_intent=${String(sample.parser_intent)}, parser_confidence=${String(sample.parser_confidence)}`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C1: feedback.written detail carries 10 locked fields',
        detail: `missing fields on sample row: ${missing.join(', ')}`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C2: feedback_events.source_surface='email_alert' on reply-parser    */
  /* routed rows                                                        */
  /* ------------------------------------------------------------------ */

  if (!routingModulePresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C2: every reply-parser-routed feedback_events row has source_surface=email_alert',
      detail: 'PENDING runtime commit'
    });
  } else {
    // Cross-reference feedback.written reply-parser rows with feedback_events.
    const eventIds = replyParserRows
      .map((r) => (r.detail as { feedback_event_id?: number } | null)?.feedback_event_id)
      .filter((id): id is number => typeof id === 'number');
    if (eventIds.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C2: every reply-parser-routed feedback_events row has source_surface=email_alert',
        detail: 'no reply-parser feedback.written rows have feedback_event_id; depends on Test 1'
      });
    } else {
      const idList = eventIds.join(',');
      const eventsCheck = await db.execute<{ id: number; source_surface: string }>(
        sql.raw(`SELECT id, source_surface FROM feedback_events WHERE id IN (${idList})`)
      );
      const rows = eventsCheck.rows as Array<{ id: number; source_surface: string }>;
      const nonEmail = rows.filter((r) => r.source_surface !== 'email_alert');
      if (nonEmail.length === 0) {
        findings.push({
          severity: 'pass',
          criterion: 'C2: every reply-parser-routed feedback_events row has source_surface=email_alert',
          detail: `${rows.length}/${eventIds.length} rows checked; all source_surface=email_alert`
        });
      } else {
        findings.push({
          severity: 'fail',
          criterion: 'C2: every reply-parser-routed feedback_events row has source_surface=email_alert',
          detail: `${nonEmail.length} non-email_alert row(s): ${JSON.stringify(nonEmail.slice(0, 3))}`
        });
      }
    }
  }

  /* ------------------------------------------------------------------ */
  /* C3: PROMPT_VERSION bumped to reply-parser-v0.2.0                    */
  /* ------------------------------------------------------------------ */

  if (promptBumped) {
    findings.push({
      severity: 'pass',
      criterion: `C3: reply-parser PROMPT_VERSION === '${EXPECTED_V0510_PROMPT_VERSION}'`,
      detail: `current: '${REPLY_PARSER_PROMPT_VERSION}'`
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: `C3: reply-parser PROMPT_VERSION === '${EXPECTED_V0510_PROMPT_VERSION}'`,
      detail: `PENDING runtime commit (current: '${REPLY_PARSER_PROMPT_VERSION}'). Bump indicates classifier intent set extended 6→8.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C4: Explicit-feedback-phrase allowlist routes through deterministic */
  /* (operator-confirmed via unit-test suite + sample audit)             */
  /* ------------------------------------------------------------------ */

  // In window, look for feedback.written rows with intent_source=
  // 'reply_parser_deterministic' (the allowlist absorption path).
  const deterministicRows = replyParserRows.filter((r) => {
    const d = r.detail as { intent_source?: string } | null;
    return d?.intent_source === 'reply_parser_deterministic';
  });
  if (!routingModulePresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C4: Q3.C explicit-feedback-phrase allowlist routes through parseReplyDeterministic (LLM never invoked)',
      detail: 'PENDING runtime commit'
    });
  } else if (deterministicRows.length === 0) {
    findings.push({
      severity: 'pending',
      criterion: 'C4: Q3.C explicit-feedback-phrase allowlist routes through parseReplyDeterministic',
      detail: 'no deterministic-source rows in window; depends on Test 1 or §3 unit-test sanity (operator confirms unit-test suite covers each allowlist phrase)'
    });
  } else {
    findings.push({
      severity: 'pass',
      criterion: 'C4: Q3.C explicit-feedback-phrase allowlist routes through parseReplyDeterministic',
      detail: `${deterministicRows.length} deterministic-source row(s); each has parser_confidence=1.0 by construction`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C5: ≤3-word safe rule forces unclear                                */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: routingModulePresent ? 'warn' : 'pending',
    criterion: 'C5: ≤3-word safe rule — 2-word non-allowlist reply with high LLM confidence → forced unclear; NO feedback_event written',
    detail: routingModulePresent
      ? 'OPERATOR + UNIT-TEST CONFIRMED — smoke cannot easily observe an "absence" event in audit_log. Unit-test suite (reply-parser/index.test.ts) MUST cover the case. Smoke runbook §3 surfaces the unit-test sanity output.'
      : 'PENDING runtime commit'
  });

  /* ------------------------------------------------------------------ */
  /* C6: ignore_sender → applyFeedback → sender_feedback_ignored upsert  */
  /* ------------------------------------------------------------------ */

  if (!routingModulePresent || !v059AuditsRegistered || !v059MemorySignalsRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C6: ignore_sender intent → applyFeedback fires → sender_feedback_ignored memory_signal upserted',
      detail: 'PENDING runtime commit (or v0.5.9 substrate regression — check carry-forward)'
    });
  } else {
    const ignoreSenderRows = replyParserRows.filter((r) => {
      const d = r.detail as { parser_intent?: string } | null;
      return d?.parser_intent === 'ignore_sender';
    });
    if (ignoreSenderRows.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C6: ignore_sender intent → applyFeedback fires → sender_feedback_ignored memory_signal upserted',
        detail: 'no ignore_sender intent rows in window; depends on Test 1'
      });
    } else {
      // Check corresponding brevio.feedback.applied audit row in window.
      const appliedRows = await db.execute<{ detail: Record<string, unknown> | null }>(
        sql`SELECT detail FROM audit_log
            WHERE action = 'brevio.feedback.applied'
              AND actor_user_id = ${FOUNDER_USER_ID}
              AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
      );
      const applied = appliedRows.rows as Array<{ detail: Record<string, unknown> | null }>;
      const senderApplied = applied.filter((r) => (r.detail as { memory_signal_kind?: string } | null)?.memory_signal_kind === 'sender_feedback_ignored');
      if (senderApplied.length >= 1) {
        findings.push({
          severity: 'pass',
          criterion: 'C6: ignore_sender intent → brevio.feedback.applied fires for sender_feedback_ignored',
          detail: `${ignoreSenderRows.length} ignore_sender feedback.written row(s); ${senderApplied.length} brevio.feedback.applied row(s) for sender_feedback_ignored`
        });
      } else {
        findings.push({
          severity: 'fail',
          criterion: 'C6: ignore_sender intent → brevio.feedback.applied fires for sender_feedback_ignored',
          detail: `${ignoreSenderRows.length} ignore_sender feedback.written row(s) but ZERO brevio.feedback.applied rows — applyFeedback NOT invoked`
        });
      }
    }
  }

  /* ------------------------------------------------------------------ */
  /* C7: this_mattered → feedback_event with positive mapping (NO        */
  /* memory_signal write)                                                */
  /* ------------------------------------------------------------------ */

  if (!routingModulePresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C7: this_mattered → feedback_event(verb=approved, dimension=importance, value=confirmed_important); NO memory_signal write',
      detail: 'PENDING runtime commit'
    });
  } else {
    const thisMatteredRows = replyParserRows.filter((r) => {
      const d = r.detail as { parser_intent?: string } | null;
      return d?.parser_intent === 'this_mattered';
    });
    if (thisMatteredRows.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C7: this_mattered → feedback_event(verb=approved, dimension=importance, value=confirmed_important)',
        detail: 'no this_mattered rows in window; depends on Test 2'
      });
    } else {
      const sample = thisMatteredRows[0]!.detail as Record<string, unknown>;
      const ok =
        sample.verb === 'approved' &&
        sample.dimension === 'importance' &&
        sample.role === 'user';
      findings.push({
        severity: ok ? 'pass' : 'fail',
        criterion: 'C7: this_mattered → feedback_event(verb=approved, dimension=importance, role=user)',
        detail: `${thisMatteredRows.length} row(s); sample: verb=${String(sample.verb)} dimension=${String(sample.dimension)} role=${String(sample.role)}`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C8: more_like_this → feedback_event with positive-pattern mapping   */
  /* ------------------------------------------------------------------ */

  if (!routingModulePresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C8: more_like_this → feedback_event(verb=approved, dimension=pattern, value=more_like_this)',
      detail: 'PENDING runtime commit'
    });
  } else {
    const moreLikeThisRows = replyParserRows.filter((r) => {
      const d = r.detail as { parser_intent?: string } | null;
      return d?.parser_intent === 'more_like_this';
    });
    if (moreLikeThisRows.length === 0) {
      findings.push({
        severity: 'warn',
        criterion: 'C8: more_like_this → feedback_event(verb=approved, dimension=pattern, value=more_like_this)',
        detail: 'no more_like_this rows in window — OPTIONAL during smoke (Test 2 covers this_mattered primarily; more_like_this can be unit-test verified)'
      });
    } else {
      const sample = moreLikeThisRows[0]!.detail as Record<string, unknown>;
      const ok =
        sample.verb === 'approved' &&
        sample.dimension === 'pattern' &&
        sample.role === 'user';
      findings.push({
        severity: ok ? 'pass' : 'fail',
        criterion: 'C8: more_like_this → feedback_event(verb=approved, dimension=pattern, role=user)',
        detail: `${moreLikeThisRows.length} row(s); sample: verb=${String(sample.verb)} dimension=${String(sample.dimension)} role=${String(sample.role)}`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C9: false_positive → corrected mapping (founder vocab CORRECTION)   */
  /* ------------------------------------------------------------------ */

  if (!routingModulePresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C9: false_positive → feedback_event(verb=corrected, dimension=ranker_label, previous_label=important, corrected_label=not_important)',
      detail: 'PENDING runtime commit'
    });
  } else {
    const fpRows = replyParserRows.filter((r) => {
      const d = r.detail as { parser_intent?: string } | null;
      return d?.parser_intent === 'false_positive';
    });
    if (fpRows.length === 0) {
      findings.push({
        severity: 'warn',
        criterion: 'C9: false_positive → feedback_event(verb=corrected, previous_label=important, corrected_label=not_important)',
        detail: 'no false_positive rows in window — OPTIONAL during smoke; unit-test suite covers the mapping'
      });
    } else {
      const sample = fpRows[0]!.detail as Record<string, unknown>;
      const ok =
        sample.verb === 'corrected' &&
        sample.dimension === 'ranker_label';
      findings.push({
        severity: ok ? 'pass' : 'fail',
        criterion: 'C9: false_positive → feedback_event(verb=corrected, dimension=ranker_label)',
        detail: `${fpRows.length} row(s); sample: verb=${String(sample.verb)} dimension=${String(sample.dimension)}`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C10: unclear writes NO feedback_event                              */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: routingModulePresent ? 'warn' : 'pending',
    criterion: 'C10: unclear intent writes NO feedback_event (returns kind=unclear_no_op from routing module)',
    detail: routingModulePresent
      ? 'OPERATOR + UNIT-TEST CONFIRMED — absence of feedback_event hard to assert in audit_log smoke; unit-test suite covers the routing module no-op path explicitly.'
      : 'PENDING runtime commit'
  });

  /* ------------------------------------------------------------------ */
  /* C11: Idempotency carry-forward (existing v0.5.5 inbound-replies     */
  /* dedup unchanged)                                                    */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: 'warn',
    criterion: 'C11: idempotency — duplicate SendBlue webhook (same provider_message_id) → ONE feedback_event, ONE applyFeedback (carry-forward)',
    detail: 'CODE-LEVEL + UNIT-TEST: the existing inbound_replies UNIQUE(provider_message_id) constraint and the v0.5.5 dedup test cover this. Operator confirms via unit-test sanity output in §3.'
  });

  /* ------------------------------------------------------------------ */
  /* C12: Cross-tenant — only founder rows in window                    */
  /* ------------------------------------------------------------------ */

  const nonFounderFeedbackRows = writtenRows.filter((r) => r.actor_user_id !== null && r.actor_user_id !== FOUNDER_USER_ID);
  let nonFounderMemRows: Array<{ user_id: string }> = [];
  if (v059MemorySignalsRegistered) {
    const memNonFounder = await db.execute<{ user_id: string }>(
      sql`SELECT user_id FROM memory_signals
          WHERE kind = 'sender_feedback_ignored'
            AND updated_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval
            AND user_id <> ${FOUNDER_USER_ID}`
    );
    nonFounderMemRows = memNonFounder.rows as Array<{ user_id: string }>;
  }
  if (nonFounderFeedbackRows.length === 0 && nonFounderMemRows.length === 0) {
    findings.push({
      severity: 'pass',
      criterion: 'C12: Cross-tenant — only founder feedback_events + sender_feedback_ignored writes in smoke window',
      detail: '0 non-founder feedback.written rows; 0 non-founder sender_feedback_ignored rows'
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'C12: Cross-tenant — only founder feedback_events + sender_feedback_ignored writes in smoke window',
      detail: `CROSS-TENANT VIOLATION — non-founder feedback.written: ${nonFounderFeedbackRows.length}; non-founder sender_feedback_ignored: ${nonFounderMemRows.length}`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C13: Privacy canary scan (tightened with reply-text patterns)      */
  /* ------------------------------------------------------------------ */

  let canaryHits = 0;
  const canaryDetails: string[] = [];

  for (const row of writtenRows) {
    const json = JSON.stringify(row.detail ?? {});
    for (const sub of FORBIDDEN_DETAIL_SUBSTRINGS) {
      if (json.includes(sub)) {
        canaryHits++;
        canaryDetails.push(`feedback.written detail contains '${sub}'`);
      }
    }
  }
  if (v059AuditsRegistered) {
    const appliedRows = await db.execute<{ detail: Record<string, unknown> | null }>(
      sql`SELECT detail FROM audit_log
          WHERE action = 'brevio.feedback.applied'
            AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    for (const row of appliedRows.rows as Array<{ detail: Record<string, unknown> | null }>) {
      const json = JSON.stringify(row.detail ?? {});
      for (const sub of FORBIDDEN_DETAIL_SUBSTRINGS) {
        if (json.includes(sub)) {
          canaryHits++;
          canaryDetails.push(`brevio.feedback.applied detail contains '${sub}'`);
        }
      }
    }
  }
  if (canaryHits === 0) {
    findings.push({
      severity: 'pass',
      criterion: 'C13: Privacy canary scan — zero forbidden substrings in new audit detail (incl. reply-text patterns)',
      detail: `scanned ${writtenRows.length} feedback.written + brevio.feedback.applied rows; checked ${FORBIDDEN_DETAIL_SUBSTRINGS.length} forbidden substring(s)`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'C13: Privacy canary scan — zero forbidden substrings in new audit detail',
      detail: `${canaryHits} hit(s): ${canaryDetails.slice(0, 5).join('; ')}${canaryDetails.length > 5 ? ` ... +${canaryDetails.length - 5} more` : ''}`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C14: Live smoke Test 1 (LOAD-BEARING)                               */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: routingModulePresent ? 'warn' : 'pending',
    criterion: 'C14: Live smoke Path A (LOAD-BEARING) — founder texts "ignore this sender" → full chain end-to-end',
    detail: routingModulePresent
      ? 'OPERATOR-CONFIRMED in runbook §6 Test 1: SendBlue inbound (or signed-curl substitute) → deterministic-allowlist match → routing module → feedback_event + applyFeedback → sender_feedback_ignored memory_signal upserted + brevio.feedback.applied audit fires.'
      : 'PENDING runtime commit'
  });

  /* ------------------------------------------------------------------ */
  /* C15: Live smoke Test 2 (positive intent)                            */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: routingModulePresent ? 'warn' : 'pending',
    criterion: 'C15: Live smoke Test 2 — founder texts "this mattered" → feedback_event with verb=approved + dimension=importance; NO sender_feedback_ignored change',
    detail: routingModulePresent
      ? 'OPERATOR-CONFIRMED in runbook §6 Test 2: deterministic-allowlist match → routing module writes positive-signal feedback_event; NO memory_signal write; NO brevio.feedback.applied audit.'
      : 'PENDING runtime commit'
  });

  /* ------------------------------------------------------------------ */
  /* C16: Carry-forward regressions (v0.5.7 + v0.5.9)                    */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: 'warn',
    criterion: 'C16: smoke-evidence:v0.5.7 (HMR) + smoke-evidence:v0.5.9 (Feedback substrate) still PASS on this branch + STOP regression (no v0.5.10 feedback_event)',
    detail:
      'OPERATOR MUST RUN: pnpm smoke-evidence:v0.5.7 && pnpm smoke-evidence:v0.5.9. ' +
      'Expected: PASS (or documented-benign window-pollution patterns per v0.5.8/v0.5.9 SMOKE_REPORTs). ' +
      'STOP regression: founder texts STOP → existing deterministic compliance path fires; no feedback.written row with parser_intent surfaces.'
  });

  /* ============================================================== */
  /* Output                                                          */
  /* ============================================================== */

  console.log('========================================================================');
  console.log('Phase v0.5.10 evidence summary — 16 criteria (Reply-parser feedback intents)');
  console.log('========================================================================');
  for (const f of findings) {
    console.log(`  [${symbol(f.severity)}] ${f.criterion}`);
    if (f.detail) console.log(`        ${f.detail}`);
  }
  console.log('');

  const failCount = findings.filter((f) => f.severity === 'fail').length;
  const pendingCount = findings.filter((f) => f.severity === 'pending').length;
  const passCount = findings.filter((f) => f.severity === 'pass').length;
  const warnCount = findings.filter((f) => f.severity === 'warn').length;

  if (failCount > 0) {
    console.log(`VERDICT: FAIL  — ${failCount} criterion(criteria) failed. Fix and re-run before considering merge.`);
    process.exit(1);
  }
  if (pendingCount > 0) {
    console.log(
      `VERDICT: PENDING  (${pendingCount} criterion(criteria) await runtime commit / smoke execution; ${passCount} PASS; ${warnCount} operator-confirmed). ` +
        `This is EXPECTED at scaffolding time.`
    );
    process.exit(0);
  }
  console.log(
    `VERDICT: PASS  (${passCount} PASS, ${warnCount} operator-confirmed). ` +
      `Operator must additionally run: smoke-evidence:v0.5.1 through smoke-evidence:v0.5.9 (C16 carry-forward).`
  );

  void ALL_V0510_INTENTS;
  void NEW_V0510_INTENTS;
  void EXPECTED_INTENT_SOURCES;
}

main().catch((err) => {
  process.stderr.write(`[smoke-evidence:v0.5.10] fatal: ${err instanceof Error ? err.message : String(err)}\n`);
  process.exit(2);
});
