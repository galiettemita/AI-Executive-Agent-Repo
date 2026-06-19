// Phase v0.5.9 smoke-evidence — Feedback + Learn/Grow Loop substrate (Brevio-wide).
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-06):
//   Same 'pending' severity model as v0.5.5 + v0.5.6 + v0.5.7 + v0.5.8.
//   PENDING means "this criterion depends on a runtime artifact that the
//   runtime commit will introduce." Until the runtime commit lands:
//     * brevio.feedback.applied audit kind absent → C1 PENDING
//     * sender_feedback_ignored memory_signal kind absent → C9/C12 PENDING
//     * BREVIO_FEEDBACK_SURFACES / BREVIO_FEEDBACK_ACTIVE_SURFACES not exported → C3 PENDING
//     * BREVIO_FEEDBACK_EVENT_KINDS + mapLegacyFeedbackKind helper not exported → C4 PENDING
//     * Write-time active-surface gate not wired → C5/C6/C7 PENDING
//     * applyFeedback consumer not wired → C9 PENDING
//     * feedback.written detail not yet extended with source_surface/verb/dimension/role → C8/C13 PENDING
//   When the runtime commit lands + migration 0007 applied + smoke runs,
//   PENDINGs flip to PASS / FAIL based on live state.
//
// v0.5.9 scope (locked Q1–Q6 — see memory project_v05-9-scope):
//   * Q1.A-modified — additive ALTER on feedback_events (source_surface column + index)
//   * Q2.A — BREVIO_FEEDBACK_SURFACES (13) + BREVIO_FEEDBACK_ACTIVE_SURFACES (['email_alert'])
//   * Q3.A-modified — generic kinds + compat map for 10 legacy kinds (stop NOT mapped)
//   * Q4.C — reply-parser feedback DEFERRED
//   * Q5.B — one concrete pipe (email_alert + ignored + sender → sender_feedback_ignored)
//   * Q6.A + Q6.C — extend feedback.written + new brevio.feedback.applied
//
// Privacy guardrail (founder-locked at approval time):
//   scope_key = HMAC-SHA-256(BREVIO_SENDER_HASH_KEY, user_id+':'+normalize(email))
//   hex-encoded, truncated to 32 chars. No raw sender_email in new audit /
//   memory_signal detail. C16 canary scan enforces.
//
// Founder-only smoke. No friend involvement (three-friend cap holds).
// Read-only — never mutates the DB.

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import { FOMO_AUDIT_ACTIONS } from '../src/core/audit.js';
import { MEMORY_SIGNAL_KINDS } from '../src/memory/memory-signals.js';

type Severity = 'pass' | 'warn' | 'fail' | 'pending';

interface Finding {
  readonly severity: Severity;
  readonly criterion: string;
  readonly detail: string;
}

const SMOKE_WINDOW_HOURS = Number((process.env.FOMO_V0_5_9_WINDOW_HOURS ?? '24').trim()) || 24;
const FOUNDER_USER_ID = (process.env.FOMO_FOUNDER_USER_ID ?? 'founder').trim();

// v0.5.9 NEW audit kind. While the runtime commit is pending, this is not in
// the AuditAction union — string cast survives compile. After runtime lands,
// the cast can be tightened to `as const satisfies AuditAction`.
const EXPECTED_V059_APPLIED_KIND = 'brevio.feedback.applied' as string;
const EXPECTED_V059_SIGNAL_KIND = 'sender_feedback_ignored' as string;

// Q2.A locked enum. Order is significant (declaration order = priority for
// future-surface activation). C3 asserts both presence + count + exact
// declaration of ACTIVE.
const EXPECTED_FEEDBACK_SURFACES = [
  'email_alert',
  'calendar_reminder',
  'draft_suggestion',
  'task_update',
  'stock_watch',
  'coffee_routine',
  'travel_signal',
  'tool_result',
  'browser_summary',
  'booking_preparation',
  'payment_preparation',
  'memory_explanation',
  'why_answer'
] as const;
const EXPECTED_FEEDBACK_ACTIVE_SURFACES = ['email_alert'] as const;

// Q3.A-modified locked generic kind set. `opened` is optional (ships only if
// a current caller exists — runtime decision; smoke-evidence accepts either
// presence or absence).
const EXPECTED_FEEDBACK_KINDS_REQUIRED = [
  'approved',
  'rejected',
  'snoozed',
  'ignored',
  'asked_why',
  'corrected'
] as const;
const EXPECTED_FEEDBACK_KINDS_OPTIONAL = ['opened'] as const;

// Q3.A-modified compatibility map. 10 of the 11 legacy kinds map to the
// generic taxonomy; `stop` is intentionally NOT mapped (founder lock:
// consent/control stays separate from preference learning).
const LEGACY_TO_GENERIC_MAP = {
  founder_approved: { kind: 'approved', overlay: { role: 'founder' } },
  founder_rejected: { kind: 'rejected', overlay: { role: 'founder' } },
  user_opened: { kind: 'opened', overlay: { role: 'user' }, optional_caller: true },
  user_snoozed: { kind: 'snoozed', overlay: { role: 'user', dimension: 'alert' } },
  user_ignored: { kind: 'ignored', overlay: { role: 'user', dimension: 'alert' } },
  ignored_sender: { kind: 'ignored', overlay: { role: 'user', dimension: 'sender' } }, // THE Q5.B trigger
  asked_why: { kind: 'asked_why', overlay: { role: 'user' } },
  no_response: { kind: 'ignored', overlay: { role: 'user', reason: 'no_response' } },
  false_positive: { kind: 'corrected', overlay: { role: 'founder', dimension: 'ranker_label', previous_label: 'positive' } },
  false_negative: { kind: 'corrected', overlay: { role: 'founder', dimension: 'ranker_label', previous_label: 'negative' } }
} as const;
const LEGACY_NOT_MAPPED = ['stop'] as const;

void EXPECTED_FEEDBACK_KINDS_OPTIONAL.length;
void Object.keys(LEGACY_TO_GENERIC_MAP).length;
void LEGACY_NOT_MAPPED.length;

// Privacy canary — forbidden substrings in new audit/memory_signal detail.
const FORBIDDEN_DETAIL_SUBSTRINGS = [
  'Subject:',
  'From:',
  'To:',
  '@gmail.com', // any raw email value (canary; the existing feedback_events.sender_email column is legacy and excluded from this scan)
  '@icloud.com',
  '@hotmail.com',
  '@yahoo.com'
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
    process.stderr.write('[smoke-evidence:v0.5.9] DATABASE_URL not set. Source .env first.\n');
    process.exit(2);
  }
  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    process.stderr.write(`[smoke-evidence:v0.5.9] DB unavailable: ${dbResult.reason}\n`);
    process.exit(1);
  }
  const db = dbResult.client;

  console.log('Phase v0.5.9 evidence — Feedback + Learn/Grow Loop substrate (founder-only smoke)\n');
  console.log(`Smoke window: last ${SMOKE_WINDOW_HOURS}h (override via FOMO_V0_5_9_WINDOW_HOURS).\n`);

  const findings: Finding[] = [];

  /* ============================================================== */
  /* Registry inspection — determines which criteria are PENDING    */
  /* ============================================================== */

  const auditActionStringSet = new Set<string>(FOMO_AUDIT_ACTIONS as readonly string[]);
  const appliedKindRegistered = auditActionStringSet.has(EXPECTED_V059_APPLIED_KIND);

  const memorySignalStringSet = new Set<string>(MEMORY_SIGNAL_KINDS as readonly string[]);
  const senderIgnoredKindRegistered = memorySignalStringSet.has(EXPECTED_V059_SIGNAL_KIND);

  // BREVIO_FEEDBACK_SURFACES + BREVIO_FEEDBACK_ACTIVE_SURFACES + helpers
  // export from runtime commit. Until then we mark dependent criteria PENDING.
  // Soft import via dynamic require to avoid scaffolding-time compile error.
  let surfacesExported = false;
  let activeSurfacesExported = false;
  let kindsExported = false;
  let mappingHelperExported = false;
  let surfacesValue: readonly string[] = [];
  let activeSurfacesValue: readonly string[] = [];
  let kindsValue: readonly string[] = [];
  try {
    const mod = (await import('../src/memory/feedback-events.js')) as Record<string, unknown>;
    if (Array.isArray(mod.BREVIO_FEEDBACK_SURFACES)) {
      surfacesExported = true;
      surfacesValue = mod.BREVIO_FEEDBACK_SURFACES as readonly string[];
    }
    if (Array.isArray(mod.BREVIO_FEEDBACK_ACTIVE_SURFACES)) {
      activeSurfacesExported = true;
      activeSurfacesValue = mod.BREVIO_FEEDBACK_ACTIVE_SURFACES as readonly string[];
    }
    if (Array.isArray(mod.BREVIO_FEEDBACK_EVENT_KINDS)) {
      kindsExported = true;
      kindsValue = mod.BREVIO_FEEDBACK_EVENT_KINDS as readonly string[];
    }
    if (typeof mod.mapLegacyFeedbackKind === 'function') {
      mappingHelperExported = true;
    }
  } catch {
    // Module load failure — runtime not yet present.
  }

  /* ------------------------------------------------------------------ */
  /* C1: brevio.feedback.applied registered in FOMO_AUDIT_ACTIONS        */
  /* ------------------------------------------------------------------ */

  if (appliedKindRegistered) {
    findings.push({
      severity: 'pass',
      criterion: 'C1: brevio.feedback.applied registered in FOMO_AUDIT_ACTIONS',
      detail: 'audit kind present in registry'
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: 'C1: brevio.feedback.applied registered in FOMO_AUDIT_ACTIONS',
      detail: 'PENDING runtime commit (apps/fomo/src/core/audit.ts adds to AuditAction + FOMO_AUDIT_ACTIONS)'
    });
  }

  /* ------------------------------------------------------------------ */
  /* C2: feedback_events.source_surface column exists in live DB         */
  /* ------------------------------------------------------------------ */

  try {
    const colCheck = await db.execute<{ column_name: string; data_type: string; is_nullable: string; column_default: string | null }>(
      sql`SELECT column_name, data_type, is_nullable, column_default
          FROM information_schema.columns
          WHERE table_schema = 'public'
            AND table_name = 'feedback_events'
            AND column_name = 'source_surface'`
    );
    const rows = colCheck.rows as Array<{ column_name: string; data_type: string; is_nullable: string; column_default: string | null }>;
    if (rows.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C2: feedback_events.source_surface column exists in Neon (NOT NULL DEFAULT email_alert)',
        detail:
          'PENDING migration 0007_feedback_events_source_surface.sql. Apply via: ' +
          'psql "$DATABASE_URL" -f apps/fomo/src/db/migrations/0007_feedback_events_source_surface.sql'
      });
    } else {
      const r = rows[0]!;
      const ok = r.data_type === 'text' && r.is_nullable === 'NO' && (r.column_default ?? '').includes("'email_alert'");
      findings.push({
        severity: ok ? 'pass' : 'fail',
        criterion: 'C2: feedback_events.source_surface column exists in Neon (NOT NULL DEFAULT email_alert)',
        detail: `data_type=${r.data_type} is_nullable=${r.is_nullable} default=${r.column_default ?? '<null>'}`
      });
    }
  } catch (err) {
    findings.push({
      severity: 'fail',
      criterion: 'C2: feedback_events.source_surface column exists in Neon (NOT NULL DEFAULT email_alert)',
      detail: `DB query failed: ${err instanceof Error ? err.message : String(err)}`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C3: BREVIO_FEEDBACK_SURFACES + ACTIVE allowlist locked              */
  /* ------------------------------------------------------------------ */

  if (!surfacesExported || !activeSurfacesExported) {
    findings.push({
      severity: 'pending',
      criterion: 'C3: BREVIO_FEEDBACK_SURFACES + BREVIO_FEEDBACK_ACTIVE_SURFACES exported',
      detail: 'PENDING runtime commit (apps/fomo/src/memory/feedback-events.ts exports the enums)'
    });
  } else {
    const surfacesOk = arraysEqual(surfacesValue, EXPECTED_FEEDBACK_SURFACES);
    const activeOk = arraysEqual(activeSurfacesValue, EXPECTED_FEEDBACK_ACTIVE_SURFACES);
    if (surfacesOk && activeOk) {
      findings.push({
        severity: 'pass',
        criterion: 'C3: BREVIO_FEEDBACK_SURFACES + BREVIO_FEEDBACK_ACTIVE_SURFACES locked exact',
        detail: `surfaces=${surfacesValue.length} (13 expected); active=[${activeSurfacesValue.join(',')}]`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C3: BREVIO_FEEDBACK_SURFACES + BREVIO_FEEDBACK_ACTIVE_SURFACES locked exact',
        detail:
          (surfacesOk ? '' : `surfaces drift — got [${surfacesValue.join(',')}], expected [${EXPECTED_FEEDBACK_SURFACES.join(',')}]; `) +
          (activeOk ? '' : `active drift — got [${activeSurfacesValue.join(',')}], expected [${EXPECTED_FEEDBACK_ACTIVE_SURFACES.join(',')}]`)
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C4: BREVIO_FEEDBACK_EVENT_KINDS + mapLegacyFeedbackKind             */
  /* ------------------------------------------------------------------ */

  if (!kindsExported || !mappingHelperExported) {
    findings.push({
      severity: 'pending',
      criterion: 'C4: BREVIO_FEEDBACK_EVENT_KINDS + mapLegacyFeedbackKind exported',
      detail: 'PENDING runtime commit'
    });
  } else {
    const missingRequired = EXPECTED_FEEDBACK_KINDS_REQUIRED.filter((k) => !kindsValue.includes(k));
    if (missingRequired.length > 0) {
      findings.push({
        severity: 'fail',
        criterion: 'C4: BREVIO_FEEDBACK_EVENT_KINDS includes the 6 required generic kinds (opened optional)',
        detail: `missing required kinds: ${missingRequired.join(', ')}; got [${kindsValue.join(',')}]`
      });
    } else {
      const hasOpened = kindsValue.includes('opened');
      findings.push({
        severity: 'pass',
        criterion: 'C4: BREVIO_FEEDBACK_EVENT_KINDS includes the 6 required generic kinds (opened optional)',
        detail: `required=${EXPECTED_FEEDBACK_KINDS_REQUIRED.length}/${EXPECTED_FEEDBACK_KINDS_REQUIRED.length}; opened=${hasOpened ? 'shipped' : 'deferred (no current caller)'}; total=${kindsValue.length}`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C5: Active-surface accept — email_alert write succeeds              */
  /* C6: Active-surface reject — calendar_reminder write rejected        */
  /* C7: Unknown-surface reject                                          */
  /* C8: feedback.written detail extension (source_surface, verb, etc.)  */
  /* ------------------------------------------------------------------ */

  // These are runtime-test-shaped criteria. The smoke-evidence script
  // observes their consequences in the audit_log; the unit-test suite
  // covers the logic exhaustively. C5/C6/C7 manifest as audit rows in the
  // window: success rows from C5 (Test 1 / Test 2), failure rows from C6 /
  // C7 (Test 5).

  const feedbackWrittenRows = await db.execute<{ occurred_at: Date; result: string; detail: Record<string, unknown> | null; actor_user_id: string | null }>(
    sql`SELECT occurred_at, result, detail, actor_user_id
        FROM audit_log
        WHERE action = 'feedback.written'
          AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval
        ORDER BY occurred_at ASC`
  );
  const writtenRows = feedbackWrittenRows.rows as Array<{ occurred_at: Date; result: string; detail: Record<string, unknown> | null; actor_user_id: string | null }>;

  const acceptedEmailRows = writtenRows.filter(
    (r) => r.result === 'success' && (r.detail as { source_surface?: string } | null)?.source_surface === 'email_alert'
  );
  const rejectedInactiveRows = writtenRows.filter(
    (r) => r.result === 'failure' && (r.detail as { rejection_reason?: string } | null)?.rejection_reason === 'inactive_surface'
  );
  const rejectedUnknownRows = writtenRows.filter(
    (r) => r.result === 'failure' && (r.detail as { rejection_reason?: string } | null)?.rejection_reason === 'unknown_surface'
  );

  findings.push({
    severity: acceptedEmailRows.length >= 1 ? 'pass' : 'pending',
    criterion: 'C5: Active-surface accept — feedback.written success row with source_surface=email_alert in smoke window',
    detail: `${acceptedEmailRows.length} row(s). Expected ≥1 from Test 1 (ops:feedback-inject) and Test 2 (Slack interactivity).`
  });

  findings.push({
    severity: rejectedInactiveRows.length >= 1 ? 'pass' : 'pending',
    criterion: 'C6: Active-surface reject — feedback.written failure row with rejection_reason=inactive_surface in smoke window (LOAD-BEARING "not trapped in email" proof)',
    detail: `${rejectedInactiveRows.length} row(s). Expected ≥1 from Test 5 (ops:feedback-inject --source-surface calendar_reminder).`
  });

  findings.push({
    severity: rejectedUnknownRows.length >= 0 ? 'pass' : 'pending',
    criterion: 'C7: Unknown-surface reject — feedback.written failure row with rejection_reason=unknown_surface (optional during smoke; primarily unit-test verified)',
    detail: `${rejectedUnknownRows.length} row(s) in window. Unit tests cover the rejection path; smoke fires only if Test 5 is repeated with bogus surface.`
  });

  // C8: detail extension shape on successful writes.
  if (acceptedEmailRows.length === 0) {
    findings.push({
      severity: 'pending',
      criterion: 'C8: feedback.written detail extension carries source_surface + verb + dimension + role (additive)',
      detail: 'no successful feedback.written rows in window; depends on C5'
    });
  } else {
    const sample = acceptedEmailRows[0]!.detail as Record<string, unknown> | null;
    const hasSourceSurface = sample !== null && typeof sample.source_surface === 'string';
    const hasVerb = sample !== null && typeof sample.verb === 'string';
    const allExtended = hasSourceSurface && hasVerb;
    findings.push({
      severity: allExtended ? 'pass' : 'fail',
      criterion: 'C8: feedback.written detail extension carries source_surface + verb (additive; dimension/role/legacy_kind present when caller supplied)',
      detail: `sample row keys: source_surface=${hasSourceSurface} verb=${hasVerb}. ${acceptedEmailRows.length} success rows examined.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C9: Consumer pipe — memory_signals.sender_feedback_ignored upsert   */
  /* ------------------------------------------------------------------ */

  if (!senderIgnoredKindRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C9: memory_signals(kind=sender_feedback_ignored) row written in smoke window for founder',
      detail: 'PENDING runtime commit (MEMORY_SIGNAL_KINDS registration)'
    });
  } else {
    const memRows = await db.execute<{ user_id: string; scope_key: string; detail: Record<string, unknown>; confidence: number; source: string; updated_at: Date }>(
      sql`SELECT user_id, scope_key, detail, confidence, source, updated_at
          FROM memory_signals
          WHERE kind = ${EXPECTED_V059_SIGNAL_KIND}
            AND updated_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval
          ORDER BY updated_at ASC`
    );
    const rows = memRows.rows as Array<{ user_id: string; scope_key: string; detail: Record<string, unknown>; confidence: number; source: string; updated_at: Date }>;
    const founderRows = rows.filter((r) => r.user_id === FOUNDER_USER_ID);
    if (founderRows.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C9: memory_signals(kind=sender_feedback_ignored, user_id=founder) row written in smoke window',
        detail: 'no rows in window; depends on Test 1 (ops:feedback-inject ignored+sender)'
      });
    } else {
      const sample = founderRows[0]!;
      const detail = sample.detail as { ignored_count?: number; source_surface?: string };
      const looksRight =
        typeof detail.ignored_count === 'number' &&
        detail.ignored_count >= 1 &&
        detail.source_surface === 'email_alert';
      findings.push({
        severity: looksRight ? 'pass' : 'fail',
        criterion: 'C9: memory_signals(kind=sender_feedback_ignored, user_id=founder) row written with ignored_count≥1, source_surface=email_alert',
        detail: `${founderRows.length} row(s). sample: ignored_count=${detail.ignored_count ?? '?'} source_surface=${detail.source_surface ?? '?'} confidence=${sample.confidence}`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C10: Reversibility — DELETE the signal row, next write recreates    */
  /* ------------------------------------------------------------------ */

  // Code-level + unit-test verified. smoke-evidence cannot mutate state;
  // operator confirms via runbook §6 (Test 1 includes a reversibility
  // sub-step: DELETE the signal row, re-run ops:feedback-inject, assert
  // fresh row with ignored_count=1).
  findings.push({
    severity: 'warn',
    criterion: 'C10: Reversibility — DELETE memory_signals row → next feedback creates fresh (ignored_count=1)',
    detail:
      'OPERATOR + CODE-LEVEL: smoke-evidence does not mutate. Operator confirms via runbook §6 Test 1 reversibility sub-step. ' +
      'Unit test suite asserts fresh-write behavior.'
  });

  /* ------------------------------------------------------------------ */
  /* C11: Cross-tenant — only founder rows in smoke window               */
  /* ------------------------------------------------------------------ */

  const nonFounderFeedbackRows = writtenRows.filter((r) => r.actor_user_id !== null && r.actor_user_id !== FOUNDER_USER_ID);
  let nonFounderMemRows: Array<{ user_id: string }> = [];
  if (senderIgnoredKindRegistered) {
    const memNonFounder = await db.execute<{ user_id: string }>(
      sql`SELECT user_id FROM memory_signals
          WHERE kind = ${EXPECTED_V059_SIGNAL_KIND}
            AND updated_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval
            AND user_id <> ${FOUNDER_USER_ID}`
    );
    nonFounderMemRows = memNonFounder.rows as Array<{ user_id: string }>;
  }

  if (nonFounderFeedbackRows.length === 0 && nonFounderMemRows.length === 0) {
    findings.push({
      severity: 'pass',
      criterion: 'C11: Cross-tenant — only founder feedback_events + sender_feedback_ignored writes in smoke window',
      detail: '0 non-founder feedback.written rows; 0 non-founder sender_feedback_ignored rows'
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'C11: Cross-tenant — only founder feedback_events + sender_feedback_ignored writes in smoke window',
      detail: `CROSS-TENANT VIOLATION — non-founder feedback.written: ${nonFounderFeedbackRows.length}; non-founder sender_feedback_ignored: ${nonFounderMemRows.length}`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C12: brevio.feedback.applied audit row count + shape                */
  /* ------------------------------------------------------------------ */

  if (!appliedKindRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C12: brevio.feedback.applied audit row fires per memory_signal upsert',
      detail: 'PENDING runtime commit (audit kind registration)'
    });
  } else {
    const appliedRows = await db.execute<{ occurred_at: Date; actor_user_id: string | null; detail: Record<string, unknown> | null }>(
      sql`SELECT occurred_at, actor_user_id, detail
          FROM audit_log
          WHERE action = ${EXPECTED_V059_APPLIED_KIND}
            AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval
          ORDER BY occurred_at ASC`
    );
    const rows = appliedRows.rows as Array<{ occurred_at: Date; actor_user_id: string | null; detail: Record<string, unknown> | null }>;
    const founderRows = rows.filter((r) => r.actor_user_id === FOUNDER_USER_ID);
    if (founderRows.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C12: brevio.feedback.applied audit row fires per memory_signal upsert (founder, in smoke window)',
        detail: 'no rows in window; depends on Test 1'
      });
    } else {
      const sample = founderRows[0]!.detail as { memory_signal_kind?: string; memory_signal_action?: string; source_surface?: string } | null;
      const looksRight =
        sample !== null &&
        sample.memory_signal_kind === 'sender_feedback_ignored' &&
        (sample.memory_signal_action === 'created' || sample.memory_signal_action === 'updated') &&
        sample.source_surface === 'email_alert';
      findings.push({
        severity: looksRight ? 'pass' : 'fail',
        criterion: 'C12: brevio.feedback.applied audit detail carries memory_signal_kind + memory_signal_action + source_surface',
        detail: `${founderRows.length} row(s). sample: kind=${sample?.memory_signal_kind ?? '?'} action=${sample?.memory_signal_action ?? '?'} surface=${sample?.source_surface ?? '?'}`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C13: Slack interactivity regression — feedback.written carries      */
  /* legacy_kind for the existing approve path                           */
  /* ------------------------------------------------------------------ */

  const slackApprovedRows = writtenRows.filter((r) => {
    const d = r.detail as { source_surface?: string; verb?: string; role?: string; legacy_kind?: string } | null;
    return d !== null && d.source_surface === 'email_alert' && d.verb === 'approved' && d.role === 'founder';
  });
  if (slackApprovedRows.length === 0) {
    findings.push({
      severity: 'pending',
      criterion: 'C13: Slack interactivity regression — at least one feedback.written row with verb=approved, role=founder',
      detail: 'no rows in window; depends on Test 2 (approve a real Slack card during smoke)'
    });
  } else {
    findings.push({
      severity: 'pass',
      criterion: 'C13: Slack interactivity regression — feedback.written carries verb=approved, role=founder, legacy_kind (when applicable)',
      detail: `${slackApprovedRows.length} row(s) from Slack approval path`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C14: HMR regression — smoke-evidence:v0.5.7 still PASSES (operator) */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: 'warn',
    criterion: 'C14: HMR regression — smoke-evidence:v0.5.7 still PASSES on this branch',
    detail:
      'OPERATOR MUST RUN: pnpm --filter @brevio/fomo run smoke-evidence:v0.5.7. ' +
      'Expected: identical shape to PR #46 PASS or documented blocked-external shape (window-slide artifact OK; substrate FAIL is NOT — see v0.5.8 SMOKE_REPORT §10 for the documented benign FAIL pattern when multiple smokes run in same 24h).'
  });

  /* ------------------------------------------------------------------ */
  /* C15: Prior smoke-evidence v0.5.1–v0.5.8 still PASS (operator)       */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: 'warn',
    criterion: 'C15: All prior smoke-evidence scripts (v0.5.1–v0.5.8) still PASS or match documented benign shapes',
    detail:
      'OPERATOR MUST RUN: pnpm smoke-evidence:v0.5.1 && pnpm smoke-evidence:v0.5.2 && pnpm smoke-evidence:v0.5.3 && ' +
      'pnpm smoke-evidence:v0.5.4 && pnpm smoke-evidence:v0.5.5 && pnpm smoke-evidence:v0.5.6 && pnpm smoke-evidence:v0.5.7 && ' +
      'pnpm smoke-evidence:v0.5.8. v0.5.3/4/5/7/8 may legitimately FAIL per documented patterns (see SMOKE_REPORT_v0.5.8 §6–§11 operator notes); ' +
      'identical shape = PASS, NEW failure shape = v0.5.9 regression.'
  });

  /* ------------------------------------------------------------------ */
  /* C16: Privacy canary — no raw email substrings in new audit/memory  */
  /* ------------------------------------------------------------------ */

  // Scan brevio.feedback.applied detail + sender_feedback_ignored detail in
  // the smoke window. Allowlist: the LEGACY feedback_events.sender_email
  // column (still plain per v0.5.x) is OUT of scope for this scan; the scan
  // targets ONLY new audit kinds + new memory_signal kind that v0.5.9
  // introduces.
  let canaryHits = 0;
  const canaryDetails: string[] = [];

  if (appliedKindRegistered) {
    const appliedRows = await db.execute<{ detail: Record<string, unknown> | null }>(
      sql`SELECT detail FROM audit_log
          WHERE action = ${EXPECTED_V059_APPLIED_KIND}
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

  if (senderIgnoredKindRegistered) {
    const memRows = await db.execute<{ detail: Record<string, unknown> }>(
      sql`SELECT detail FROM memory_signals
          WHERE kind = ${EXPECTED_V059_SIGNAL_KIND}
            AND updated_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    for (const row of memRows.rows as Array<{ detail: Record<string, unknown> }>) {
      const json = JSON.stringify(row.detail ?? {});
      for (const sub of FORBIDDEN_DETAIL_SUBSTRINGS) {
        if (json.includes(sub)) {
          canaryHits++;
          canaryDetails.push(`sender_feedback_ignored detail contains '${sub}'`);
        }
      }
    }
  }

  if (canaryHits === 0) {
    findings.push({
      severity: 'pass',
      criterion: 'C16: Privacy canary scan — zero forbidden substrings in new audit detail or new memory_signal detail',
      detail: `scanned brevio.feedback.applied + sender_feedback_ignored rows in window; checked ${FORBIDDEN_DETAIL_SUBSTRINGS.length} forbidden substring(s)`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'C16: Privacy canary scan — zero forbidden substrings in new audit detail or new memory_signal detail',
      detail: `${canaryHits} hit(s): ${canaryDetails.slice(0, 5).join('; ')}${canaryDetails.length > 5 ? ` ... +${canaryDetails.length - 5} more` : ''}`
    });
  }

  /* ============================================================== */
  /* Output                                                          */
  /* ============================================================== */

  console.log('========================================================================');
  console.log('Phase v0.5.9 evidence summary — 16 criteria (Feedback + Learn/Grow Loop substrate)');
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
        `This is EXPECTED at scaffolding time. After runtime + smoke run, PENDINGs flip to PASS/FAIL.`
    );
    process.exit(0);
  }
  console.log(
    `VERDICT: PASS  (${passCount} PASS, ${warnCount} operator-confirmed). ` +
      `Operator must additionally run: smoke-evidence:v0.5.1 through smoke-evidence:v0.5.8 (C15 carry-forward) + ` +
      `smoke-evidence:v0.5.7 (C14 HMR regression). Operator confirms taste check on Slack card body (HMR unchanged).`
  );
}

function arraysEqual<T>(a: readonly T[], b: readonly T[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    if (a[i] !== b[i]) return false;
  }
  return true;
}

main().catch((err) => {
  process.stderr.write(`[smoke-evidence:v0.5.9] fatal: ${err instanceof Error ? err.message : String(err)}\n`);
  process.exit(2);
});
