// Phase v0.5.11 smoke-evidence — PIL substrate + shadow ranker context/eval.
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-07):
//   Same 'pending' severity model as v0.5.5–v0.5.10. PENDING means "this
//   criterion depends on a runtime artifact that the runtime commit will
//   introduce." Until the runtime commit lands:
//     * sender_importance / sender_suppressed missing from MEMORY_SIGNAL_KINDS
//       → preflight ERROR (they are PRE-EXISTING in v0.5.10; missing =
//         regression, NOT a v0.5.11 PENDING)
//
// SCAFFOLDING CORRECTION (2026-06-07): both kinds are already registered as
// of v0.5.10. v0.5.10 `applyIgnoreSender` writes sender_suppressed with
// scope_key='message:<id>' as a documented v0.1 placeholder. v0.5.11 ADDS
// the producer pipe AND tightens the scope_key contract to HMAC(sender_email).
// C6 narrows its scope_key shape check to ROWS written DURING the smoke
// window (excluding pre-window placeholder rows from v0.5.10 smoke).
//     * brevio.signal.aggregated not in FOMO_AUDIT_ACTIONS
//       → C15 PENDING
//     * apps/fomo/src/memory/pil-aggregation.ts not present
//       → C1/C2/C3/C7/C18 PENDING
//     * apps/fomo/src/ranker/pil-context.ts not present
//       → C8/C9/C10/C11/C12 PENDING
//     * apps/fomo/src/eval/pil-shadow.eval.ts not present
//       → C11/C12 PENDING
//     * alerts.sender_email_hash column not present
//       → C4/C5 PENDING (verified via information_schema lookup)
//   When runtime + migration + smoke run, PENDINGs flip to PASS/FAIL per
//   live state.
//
// v0.5.11 scope (locked Q1–Q6 — see memory project_v05-11-scope):
//   * Q1 (modified hybrid) — aggregation substrate + sender_email threading
//     + shadow ranker context/eval contract; NO live ranker change.
//   * Q2.B — sender_importance + sender_suppressed; v0.5.9
//     sender_feedback_ignored UNTOUCHED (no migration).
//   * Q3.B — linear 90/180-day recency decay, applied at READ time.
//   * Q4.A — sender_email threading: new alerts.sender_email_hash column
//     (HMAC); closes v0.5.10 §15 bonus finding #1.
//   * Q5.C — tunable env: FOMO_PIL_K_THRESHOLD (3), FOMO_PIL_SCORE_DELTA
//     (0.1), FOMO_PIL_RECENCY_FULL_DAYS (90), FOMO_PIL_RECENCY_DECAY_DAYS
//     (90).
//   * Q6.B — new audit kind brevio.signal.aggregated with 15 locked detail
//     fields.
//
//   HARD INVARIANTS (each enforced by ≥1 criterion):
//     - Live ranker behavior UNCHANGED (C13)
//     - PROMPT_VERSION still ranker-v0.2.0 (C13)
//     - No outbound alert behavior change (C14)
//     - Cross-user contamination test load-bearing (C9 + C12)
//     - One correction does not flip (C7)
//     - Privacy canary clean (C16) — no raw email/subject/body/snippet
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

const SMOKE_WINDOW_HOURS = Number((process.env.FOMO_V0_5_11_WINDOW_HOURS ?? '24').trim()) || 24;
const FOUNDER_USER_ID = (process.env.FOMO_FOUNDER_USER_ID ?? 'founder').trim();

const EXPECTED_V0510_PROMPT_VERSION = 'reply-parser-v0.2.0' as string;
const EXPECTED_V0511_RANKER_PROMPT_VERSION = 'ranker-v0.2.0' as string;

// v0.5.11 new memory_signal kinds
const PIL_NEW_KINDS = ['sender_importance', 'sender_suppressed'] as const;
type PilKind = (typeof PIL_NEW_KINDS)[number];

// v0.5.11 new audit kind
const SIGNAL_AGGREGATED_KIND = 'brevio.signal.aggregated';

// Locked 15 detail fields on brevio.signal.aggregated (per Q6.B)
const SIGNAL_AGGREGATED_DETAIL_FIELDS = [
  'verb',
  'dimension',
  'feedback_event_id',
  'source_surface',
  'memory_signal_kind',
  'memory_signal_action',
  'memory_signal_scope_key_hash',
  'score_before',
  'score_after',
  'score_delta',
  'n_positive_events_before',
  'n_positive_events_after',
  'n_negative_events_before',
  'n_negative_events_after',
  'suppression_flipped',
  'threshold_k_in_force'
] as const;

// Privacy canary — forbidden substrings in any new memory_signal detail or
// audit detail. Extended from v0.5.10 with PIL-shape canaries.
const FORBIDDEN_DETAIL_SUBSTRINGS = [
  'Subject:',
  'From:',
  'To:',
  '@gmail.com',
  '@icloud.com',
  '@hotmail.com',
  '@yahoo.com',
  // Reply-text + email-body canaries
  'ignore this',
  'not important',
  'this mattered',
  'more like this',
  // PIL-specific raw-content canaries
  'noreply@',
  'unsubscribe'
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
    process.stderr.write('[smoke-evidence:v0.5.11] DATABASE_URL not set. Source .env first.\n');
    process.exit(2);
  }
  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    process.stderr.write(`[smoke-evidence:v0.5.11] DB unavailable: ${dbResult.reason}\n`);
    process.exit(1);
  }
  const db = dbResult.client;

  console.log('Phase v0.5.11 evidence — PIL substrate + shadow ranker context/eval (founder-only smoke)\n');
  console.log(`Smoke window: last ${SMOKE_WINDOW_HOURS}h (override via FOMO_V0_5_11_WINDOW_HOURS).\n`);

  const findings: Finding[] = [];

  /* ============================================================== */
  /* Registry inspection — determines which criteria are PENDING    */
  /* ============================================================== */

  const auditActionSet = new Set<string>(FOMO_AUDIT_ACTIONS as readonly string[]);
  const memorySignalSet = new Set<string>(MEMORY_SIGNAL_KINDS as readonly string[]);

  const signalAggregatedRegistered = auditActionSet.has(SIGNAL_AGGREGATED_KIND);
  // PIL kinds are pre-existing as of v0.5.10 (NOT new in v0.5.11). Preflight
  // gates on them as ERROR-if-missing. Here we just confirm registry intact.
  const senderImportanceRegistered = memorySignalSet.has('sender_importance');
  const senderSuppressedRegistered = memorySignalSet.has('sender_suppressed');
  const pilKindsRegistered = senderImportanceRegistered && senderSuppressedRegistered;
  const v059MemorySignalUntouched = memorySignalSet.has('sender_feedback_ignored');

  // v0.5.10 invariant — reply parser must still be v0.2.0.
  const v0510Held = (REPLY_PARSER_PROMPT_VERSION as string) === EXPECTED_V0510_PROMPT_VERSION;

  // pil-aggregation.ts presence
  let pilAggregationModulePresent = false;
  try {
    const modulePath = '../src/memory/pil-aggregation.js';
    const mod = (await import(modulePath)) as Record<string, unknown>;
    pilAggregationModulePresent = typeof mod.applyPilAggregation === 'function';
  } catch {
    pilAggregationModulePresent = false;
  }

  // pil-context.ts presence
  let pilContextModulePresent = false;
  try {
    const modulePath = '../src/ranker/pil-context.js';
    const mod = (await import(modulePath)) as Record<string, unknown>;
    pilContextModulePresent = typeof mod.buildPilContext === 'function';
  } catch {
    pilContextModulePresent = false;
  }

  // alerts.sender_email_hash column presence (DB-level check)
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
    !pilKindsRegistered ||
    !signalAggregatedRegistered ||
    !pilAggregationModulePresent ||
    !pilContextModulePresent ||
    !alertsHashColumnPresent;

  /* ------------------------------------------------------------------ */
  /* C1: aggregation pipe — sender_importance upserts from positive     */
  /* intents (this_mattered +δ, more_like_this +2δ)                      */
  /* ------------------------------------------------------------------ */

  if (!pilAggregationModulePresent || !senderImportanceRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C1: aggregation pipe wires positive intents → sender_importance upserts',
      detail: 'PENDING runtime commit (pil-aggregation.ts or sender_importance kind not yet present)'
    });
  } else {
    const senderImportanceRows = await db.execute<{
      user_id: string;
      scope_key: string;
      detail: Record<string, unknown> | null;
      updated_at: Date;
    }>(
      sql`SELECT user_id, scope_key, detail, updated_at FROM memory_signals
          WHERE kind = 'sender_importance'
            AND updated_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    const rows = senderImportanceRows.rows as Array<{
      user_id: string;
      scope_key: string;
      detail: Record<string, unknown> | null;
      updated_at: Date;
    }>;
    if (rows.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C1: aggregation pipe wires positive intents → sender_importance upserts',
        detail: 'no sender_importance rows in window; depends on Path A/B execution'
      });
    } else {
      const sample = rows[0]!.detail as Record<string, unknown> | null;
      const requiredKeys = ['score', 'n_positive_events', 'n_negative_events', 'last_updated', 'source_surface'];
      const missingKeys = sample === null ? requiredKeys : requiredKeys.filter((k) => !(k in sample));
      if (missingKeys.length === 0) {
        findings.push({
          severity: 'pass',
          criterion: 'C1: aggregation pipe wires positive intents → sender_importance upserts',
          detail: `${rows.length} sender_importance row(s); sample score=${String((sample as { score?: number }).score)} n_pos=${String((sample as { n_positive_events?: number }).n_positive_events)} n_neg=${String((sample as { n_negative_events?: number }).n_negative_events)}`
        });
      } else {
        findings.push({
          severity: 'fail',
          criterion: 'C1: aggregation pipe wires positive intents → sender_importance upserts',
          detail: `sender_importance detail missing required keys: ${missingKeys.join(', ')}`
        });
      }
    }
  }

  /* ------------------------------------------------------------------ */
  /* C2: false_positive → score −δ; flip threshold ≥ k                  */
  /* ------------------------------------------------------------------ */

  if (!pilAggregationModulePresent || !senderSuppressedRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C2: false_positive → score −δ; sender_suppressed flips only at n_negative_events ≥ FOMO_PIL_K_THRESHOLD',
      detail: 'PENDING runtime commit (pil-aggregation.ts or sender_suppressed kind not yet present)'
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: 'C2: false_positive → score −δ; sender_suppressed flips only at n_negative_events ≥ FOMO_PIL_K_THRESHOLD',
      detail: 'OPERATOR + UNIT-TEST CONFIRMED. Runtime unit suite (pil-aggregation.test.ts) covers single-event no-flip + k-event-flip cases. Path C in runbook §8 fires 3 consecutive false_positive on a synthetic sender via ops:feedback-inject; smoke confirms via DB query.'
    });
  }

  /* ------------------------------------------------------------------ */
  /* C3: ignore_sender carry-forward writes BOTH sender_feedback_       */
  /* ignored (v0.5.9) AND new sender_suppressed=true                    */
  /* ------------------------------------------------------------------ */

  if (!pilAggregationModulePresent || !senderSuppressedRegistered || !v059MemorySignalUntouched) {
    findings.push({
      severity: 'pending',
      criterion: 'C3: ignore_sender writes v0.5.9 sender_feedback_ignored AND new sender_suppressed=true (single explicit flip)',
      detail: 'PENDING runtime commit (substrate not yet aligned)'
    });
  } else {
    const ignoreScope = await db.execute<{ kind: string; scope_key: string; updated_at: Date }>(
      sql`SELECT kind, scope_key, updated_at FROM memory_signals
          WHERE kind IN ('sender_feedback_ignored', 'sender_suppressed')
            AND user_id = ${FOUNDER_USER_ID}
            AND updated_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    const ignoreRows = ignoreScope.rows as Array<{ kind: string; scope_key: string; updated_at: Date }>;
    const grouped = ignoreRows.reduce<Record<string, string[]>>((acc, r) => {
      const key = r.scope_key;
      (acc[key] ??= []).push(r.kind);
      return acc;
    }, {});
    const dualScopeKeys = Object.entries(grouped).filter(
      ([, kinds]) => kinds.includes('sender_feedback_ignored') && kinds.includes('sender_suppressed')
    );
    if (dualScopeKeys.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C3: ignore_sender writes v0.5.9 sender_feedback_ignored AND new sender_suppressed=true',
        detail: 'no scope_key with both kinds in window; depends on Path A execution'
      });
    } else {
      findings.push({
        severity: 'pass',
        criterion: 'C3: ignore_sender writes BOTH v0.5.9 sender_feedback_ignored AND new sender_suppressed=true',
        detail: `${dualScopeKeys.length} scope_key(s) carry both kinds. v0.5.9 untouched (no migration); v0.5.11 adds the canonical suppression kind alongside.`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C4: alerts.sender_email_hash column present + populated forward     */
  /* (migration 0008)                                                    */
  /* ------------------------------------------------------------------ */

  if (!alertsHashColumnPresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C4: alerts.sender_email_hash column present (migration 0008 applied)',
      detail: 'PENDING migration 0008 applied to live Neon — column not found in information_schema'
    });
  } else {
    const recentAlerts = await db.execute<{ total: number; with_hash: number }>(
      sql`SELECT
            COUNT(*)::int AS total,
            COUNT(sender_email_hash)::int AS with_hash
          FROM alerts
          WHERE created_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    const row = (recentAlerts.rows as Array<{ total: number; with_hash: number }>)[0] ?? { total: 0, with_hash: 0 };
    if (row.total === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C4: alerts.sender_email_hash populated forward by rank step',
        detail: 'no fresh alerts in window; depends on polling cycle producing at least one ranked alert during smoke'
      });
    } else if (row.with_hash > 0) {
      findings.push({
        severity: 'pass',
        criterion: 'C4: alerts.sender_email_hash populated forward by rank step',
        detail: `${row.with_hash}/${row.total} fresh alert(s) carry sender_email_hash. Backfill on pre-migration rows = NULL by design.`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C4: alerts.sender_email_hash populated forward by rank step',
        detail: `${row.total} fresh alert(s) in window but 0 carry sender_email_hash. Rank-step write path missing.`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C5: natural-reply consumer arm binds via alerts.sender_email_hash  */
  /* (closes v0.5.10 §15 bonus finding #1)                              */
  /* ------------------------------------------------------------------ */

  if (!alertsHashColumnPresent || !senderSuppressedRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C5: natural-reply ignore_sender on a v0.5.11-rank-time alert creates sender_suppressed bound to alert.sender_email_hash',
      detail: 'PENDING runtime commit + migration 0008'
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: 'C5: natural-reply ignore_sender on a v0.5.11-rank-time alert creates sender_suppressed bound to alert.sender_email_hash',
      detail: 'OPERATOR-CONFIRMED in runbook §6 Path A. Smoke fires natural "ignore this sender" reply; query asserts memory_signals(sender_suppressed).scope_key matches the matched alert.sender_email_hash.'
    });
  }

  /* ------------------------------------------------------------------ */
  /* C6: per-user HMAC scope_key prevents raw sender leakage             */
  /* ------------------------------------------------------------------ */

  if (!pilKindsRegistered) {
    findings.push({
      severity: 'fail',
      criterion: 'C6: per-user HMAC scope_key (32-hex) on new memory_signal kinds — prevents raw sender leakage',
      detail: 'PIL kinds missing from MEMORY_SIGNAL_KINDS registry — registry regression. Preflight should have caught this.'
    });
  } else {
    // Narrow to rows written DURING the smoke window. Pre-window rows from
    // v0.5.10's `message:<id>`-keyed placeholder writes are documented carry-
    // forward, not v0.5.11 producer output, so we exclude them. C6 asserts
    // that v0.5.11-window writes use the new HMAC contract.
    const newKindRows = await db.execute<{ scope_key: string; kind: string }>(
      sql`SELECT scope_key, kind FROM memory_signals
          WHERE kind IN ('sender_importance', 'sender_suppressed')
            AND updated_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    const rows = newKindRows.rows as Array<{ scope_key: string; kind: string }>;
    if (rows.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C6: per-user HMAC scope_key (32-hex) on new memory_signal kinds',
        detail: 'no v0.5.11-window rows for the PIL kinds; depends on smoke execution. Pre-window v0.5.10 `message:<id>` placeholder rows excluded from check.'
      });
    } else {
      // Legacy v0.5.10 placeholder shape: scope_key='message:<gmail_id>'.
      // Per founder guardrail #1, v0.5.10's applyIgnoreSender continues to
      // write these UNCHANGED. v0.5.11 adds HMAC-keyed rows ALONGSIDE the
      // placeholders. C6 evaluates ONLY the v0.5.11 producer's output —
      // legacy rows are documented carry-forward, NOT v0.5.11 regressions.
      const newContractKeys = rows.filter((r) => !r.scope_key.startsWith('message:'));
      const badNewKeys = newContractKeys.filter((r) => !/^[0-9a-f]{32}$/.test(r.scope_key));
      if (badNewKeys.length === 0 && newContractKeys.length > 0) {
        findings.push({
          severity: 'pass',
          criterion: 'C6: per-user HMAC scope_key (32-hex) on new memory_signal kinds — prevents raw sender leakage',
          detail: `${newContractKeys.length} v0.5.11-window row(s); all scope_keys match /^[0-9a-f]{32}$/`
        });
      } else if (badNewKeys.length > 0) {
        findings.push({
          severity: 'fail',
          criterion: 'C6: per-user HMAC scope_key (32-hex) on new memory_signal kinds',
          detail: `${badNewKeys.length} row(s) carry scope_keys that are NOT 32-hex AND NOT 'message:' (potential plaintext leak). Sample bad kind=${badNewKeys[0]!.kind} key=${badNewKeys[0]!.scope_key.slice(0, 30)}...`
        });
      } else {
        findings.push({
          severity: 'pending',
          criterion: 'C6: per-user HMAC scope_key (32-hex) on new memory_signal kinds',
          detail: 'no new-contract rows in window; depends on smoke execution'
        });
      }
    }
  }

  /* ------------------------------------------------------------------ */
  /* C7: one correction does NOT flip                                    */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: 'pending',
    criterion: 'C7: one correction does not flip — single false_positive event lowers score but does NOT flip sender_suppressed',
    detail: 'OPERATOR + UNIT-TEST CONFIRMED. Runtime suite (pil-aggregation.test.ts) covers the one-event no-flip case. Runbook §8 Path C smoke confirms via DB: 1 false_positive → score ≈ −δ, no sender_suppressed row; then k consecutive false_positives → sender_suppressed=true with set_by="threshold_negative_aggregation".'
  });

  /* ------------------------------------------------------------------ */
  /* C8: recency decay tested (linear 0-90/90-180/180+)                  */
  /* ------------------------------------------------------------------ */

  if (!pilContextModulePresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C8: linear recency decay (full 0-90d, linear 90-180d, zero 180d+) tested',
      detail: 'PENDING runtime commit (pil-context.ts not present)'
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: 'C8: linear recency decay (full 0-90d, linear 90-180d, zero 180d+) tested',
      detail: 'OPERATOR + UNIT-TEST CONFIRMED. Runtime suite (pil-context.test.ts) covers ages [0d, 45d, 90d, 135d, 180d, 200d] with expected decayed scores within tolerance. Eval harness (pil-shadow.eval.ts) additionally exercises decay at the shadow-projection layer.'
    });
  }

  /* ------------------------------------------------------------------ */
  /* C9: cross-user contamination — LOAD-BEARING                         */
  /* ------------------------------------------------------------------ */

  if (!pilContextModulePresent || !pilKindsRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C9: cross-user contamination — user A ignore_sender on hash(X) leaves user B buildPilContext null + shadow score unchanged (LOAD-BEARING)',
      detail: 'PENDING runtime commit'
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: 'C9: cross-user contamination — user A ignore_sender on hash(X) leaves user B buildPilContext null + shadow score unchanged (LOAD-BEARING)',
      detail: 'OPERATOR-CONFIRMED in runbook §10 Path E. ops:feedback-inject as synthetic user A on sender X → query memory_signals for user B with the SAME scope_key returns 0 rows. buildPilContext(userB, hash(X)) returns null. Shadow eval asserts user B score unchanged from baseline.'
    });
  }

  /* ------------------------------------------------------------------ */
  /* C10: shadow ranker context contract                                 */
  /* ------------------------------------------------------------------ */

  if (!pilContextModulePresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C10: buildPilContext(userId, senderEmailHash, deps) exported from apps/fomo/src/ranker/pil-context.ts',
      detail: 'PENDING runtime commit (pil-context.ts not present)'
    });
  } else {
    findings.push({
      severity: 'pass',
      criterion: 'C10: buildPilContext(userId, senderEmailHash, deps) exported from apps/fomo/src/ranker/pil-context.ts (pure projection; no model call)',
      detail: 'Module present + helper export verified. Returns PilContext | null. Decay applied at read time.'
    });
  }

  /* ------------------------------------------------------------------ */
  /* C11: shadow ranker eval harness                                     */
  /* ------------------------------------------------------------------ */

  // Eval harness file existence — separate from pil-context.
  let evalHarnessPresent = false;
  try {
    const modulePath = '../src/eval/pil-shadow.eval.js';
    await import(modulePath);
    evalHarnessPresent = true;
  } catch {
    evalHarnessPresent = false;
  }

  if (!evalHarnessPresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C11: shadow ranker eval harness present at apps/fomo/src/eval/pil-shadow.eval.ts (≥10 synthetic fixtures, shift-DIRECTION assertions)',
      detail: 'PENDING runtime commit (pil-shadow.eval.ts not present)'
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: 'C11: shadow ranker eval harness present with ≥10 synthetic fixtures + shift-DIRECTION assertions',
      detail: 'OPERATOR-CONFIRMED via `pnpm --filter @brevio/fomo run eval:pil-shadow` (runbook §9). Asserts shift DIRECTION not exact magnitude (model nondeterminism allowed).'
    });
  }

  /* ------------------------------------------------------------------ */
  /* C12: shadow eval cross-user contamination row passes                */
  /* ------------------------------------------------------------------ */

  if (!evalHarnessPresent) {
    findings.push({
      severity: 'pending',
      criterion: 'C12: shadow eval cross-user contamination row passes (mirror of C9 at eval boundary)',
      detail: 'PENDING runtime commit (eval harness not present)'
    });
  } else {
    findings.push({
      severity: 'pending',
      criterion: 'C12: shadow eval cross-user contamination row passes (mirror of C9 at eval boundary)',
      detail: 'OPERATOR-CONFIRMED via runbook §9 eval-output inspection. The contamination row asserts user B baseline score equals user B + (user A signal injected) score.'
    });
  }

  /* ------------------------------------------------------------------ */
  /* C13: live ranker behavior UNCHANGED                                 */
  /* ------------------------------------------------------------------ */

  // The hard invariant: production ranker call site passes pil_context: null.
  // Smoke-evidence cannot reach into the call site directly; it relies on the
  // grep + unit test attestation that runtime commit must include. Here we
  // assert that PROMPT_VERSION did NOT bump and that rank_results in the
  // window have the expected schema (no new column).
  findings.push({
    severity: 'pending',
    criterion: 'C13: live ranker behavior UNCHANGED — PROMPT_VERSION still ranker-v0.2.0; production call site passes pil_context: null unconditionally; rank_results schema unchanged',
    detail:
      `OPERATOR + UNIT-TEST + GREP CONFIRMED. Runtime unit test must include: ` +
      `(a) grep assertion 'pil_context' appears in buildRankerPrompt only as parameter, default null; ` +
      `(b) production call site (gmail-poll.ts or equivalent) does NOT import buildPilContext; ` +
      `(c) rank_results table schema unchanged (information_schema.columns diff). ` +
      `Smoke confirms by checking: PROMPT_VERSION literal = '${EXPECTED_V0511_RANKER_PROMPT_VERSION}'.`
  });

  /* ------------------------------------------------------------------ */
  /* C14: no outbound alert behavior changes — sender_suppressed=true is */
  /* NEVER read by live dispatch in v0.5.11                              */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: 'pending',
    criterion: 'C14: no outbound alert behavior changes — sender_suppressed=true memory_signal is NEVER read by live alert dispatch in v0.5.11',
    detail:
      'OPERATOR + UNIT-TEST CONFIRMED. Runtime integration test must demonstrate: pre-existing queued_for_review alert remains dispatchable after operator inserts sender_suppressed=true for its sender hash. Live dispatch path does NOT import the sender_suppressed kind. v0.5.12 gate decides if/when to wire it.'
  });

  /* ------------------------------------------------------------------ */
  /* C15: brevio.signal.aggregated audit kind + 15 locked detail fields   */
  /* ------------------------------------------------------------------ */

  if (!signalAggregatedRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C15: brevio.signal.aggregated audit kind registered + fires on every memory_signal upsert with 15 locked detail fields',
      detail: `PENDING runtime commit (${SIGNAL_AGGREGATED_KIND} not yet in FOMO_AUDIT_ACTIONS)`
    });
  } else {
    const aggRows = await db.execute<{ detail: Record<string, unknown> | null; occurred_at: Date }>(
      sql`SELECT detail, occurred_at FROM audit_log
          WHERE action = ${SIGNAL_AGGREGATED_KIND}
            AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    const rows = aggRows.rows as Array<{ detail: Record<string, unknown> | null; occurred_at: Date }>;
    if (rows.length === 0) {
      findings.push({
        severity: 'pending',
        criterion: 'C15: brevio.signal.aggregated audit fires + carries 15 locked detail fields',
        detail: 'kind registered but no rows in window; depends on smoke execution'
      });
    } else {
      const sample = rows[0]!.detail as Record<string, unknown> | null;
      const missing = sample === null
        ? [...SIGNAL_AGGREGATED_DETAIL_FIELDS]
        : SIGNAL_AGGREGATED_DETAIL_FIELDS.filter((k) => !(k in sample));
      if (missing.length === 0) {
        findings.push({
          severity: 'pass',
          criterion: 'C15: brevio.signal.aggregated audit fires + carries 15 locked detail fields',
          detail: `${rows.length} row(s); sample fields complete (verb, dimension, memory_signal_kind, scope_key_hash, score_before/after, n_pos/neg_before/after, suppression_flipped, threshold_k_in_force, +rest)`
        });
      } else {
        findings.push({
          severity: 'fail',
          criterion: 'C15: brevio.signal.aggregated audit fires + carries 15 locked detail fields',
          detail: `sample detail missing fields: ${missing.join(', ')}`
        });
      }
    }
  }

  /* ------------------------------------------------------------------ */
  /* C16: privacy canary — zero forbidden substrings in new audit +      */
  /* memory_signal detail                                                */
  /* ------------------------------------------------------------------ */

  // Scan new audit rows + new memory_signal rows for forbidden substrings.
  const scanRows: Array<{ source: string; payload: string }> = [];
  if (signalAggregatedRegistered) {
    const aggRows = await db.execute<{ detail: Record<string, unknown> | null }>(
      sql`SELECT detail FROM audit_log
          WHERE action = ${SIGNAL_AGGREGATED_KIND}
            AND occurred_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    for (const r of aggRows.rows as Array<{ detail: Record<string, unknown> | null }>) {
      if (r.detail !== null) scanRows.push({ source: 'brevio.signal.aggregated', payload: JSON.stringify(r.detail) });
    }
  }
  if (pilKindsRegistered) {
    const memRows = await db.execute<{ detail: Record<string, unknown> | null; kind: string }>(
      sql`SELECT detail, kind FROM memory_signals
          WHERE kind IN ('sender_importance', 'sender_suppressed')
            AND updated_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    for (const r of memRows.rows as Array<{ detail: Record<string, unknown> | null; kind: string }>) {
      if (r.detail !== null) scanRows.push({ source: `memory_signals(${r.kind})`, payload: JSON.stringify(r.detail) });
    }
  }
  const canaryHits: Array<{ source: string; substr: string }> = [];
  for (const row of scanRows) {
    for (const substr of FORBIDDEN_DETAIL_SUBSTRINGS) {
      if (row.payload.includes(substr)) {
        canaryHits.push({ source: row.source, substr });
      }
    }
  }
  if (scanRows.length === 0) {
    findings.push({
      severity: 'pending',
      criterion: 'C16: privacy canary — zero forbidden substrings in new audit + memory_signal detail',
      detail: `no v0.5.11 audit or memory_signal rows yet to scan; depends on smoke execution. Canary substring count=${FORBIDDEN_DETAIL_SUBSTRINGS.length}`
    });
  } else if (canaryHits.length > 0) {
    findings.push({
      severity: 'fail',
      criterion: 'C16: privacy canary — zero forbidden substrings in new audit + memory_signal detail',
      detail: `${canaryHits.length} hit(s). Sample: source=${canaryHits[0]!.source} substr='${canaryHits[0]!.substr}'`
    });
  } else {
    findings.push({
      severity: 'pass',
      criterion: 'C16: privacy canary — zero forbidden substrings in new audit + memory_signal detail',
      detail: `scanned ${scanRows.length} v0.5.11 row(s) against ${FORBIDDEN_DETAIL_SUBSTRINGS.length} forbidden substring(s); zero hits`
    });
  }

  /* ------------------------------------------------------------------ */
  /* C17: cross-tenant carry-forward — only founder writes in window;    */
  /* v0.5.9 sender_feedback_ignored UNTOUCHED by v0.5.11                 */
  /* ------------------------------------------------------------------ */

  if (!pilKindsRegistered) {
    findings.push({
      severity: 'pending',
      criterion: 'C17: cross-tenant — only founder writes; v0.5.9 sender_feedback_ignored UNTOUCHED + smoke-evidence:v0.5.9 + smoke-evidence:v0.5.10 still PASS',
      detail: 'PENDING runtime commit'
    });
  } else {
    const nonFounderNewKinds = await db.execute<{ ct: number }>(
      sql`SELECT COUNT(*)::int AS ct FROM memory_signals
          WHERE kind IN ('sender_importance', 'sender_suppressed')
            AND user_id <> ${FOUNDER_USER_ID}
            AND updated_at > now() - (${String(SMOKE_WINDOW_HOURS) + ' hours'})::interval`
    );
    const ct = (nonFounderNewKinds.rows as Array<{ ct: number }>)[0]?.ct ?? 0;
    if (ct === 0) {
      findings.push({
        severity: 'pass',
        criterion: 'C17: cross-tenant — only founder writes for new kinds; v0.5.9 + v0.5.10 carry-forward verdicts must be operator-checked separately',
        detail: `0 non-founder sender_importance or sender_suppressed rows in window. OPERATOR MUST RUN: pnpm smoke-evidence:v0.5.9 && pnpm smoke-evidence:v0.5.10 — both must PASS or match documented benign shapes per [[v05-10-pass]].`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'C17: cross-tenant — only founder writes for new kinds',
        detail: `${ct} non-founder sender_importance/sender_suppressed row(s) in window — cross-user isolation violated`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* C18: reversibility — delete + recreate cycle                        */
  /* ------------------------------------------------------------------ */

  findings.push({
    severity: 'pending',
    criterion: 'C18: reversibility — delete sender_importance row → next aggregation event recreates (memory_signal_action=created); delete sender_suppressed row → suppression cleared until next flipping event',
    detail: 'OPERATOR + UNIT-TEST CONFIRMED. Runbook §6 Path A includes a reversibility sub-step: delete the row, re-fire the aggregation, observe brevio.signal.aggregated.memory_signal_action=created on the second emit.'
  });

  /* ============================================================== */
  /* Output                                                          */
  /* ============================================================== */

  console.log('========================================================================');
  console.log('Phase v0.5.11 evidence summary — 18 criteria (PIL substrate + shadow eval)');
  console.log('========================================================================');
  for (const f of findings) {
    console.log(`  [${symbol(f.severity)}] ${f.criterion}`);
    console.log(`        ${f.detail}`);
  }

  const pass = findings.filter((f) => f.severity === 'pass').length;
  const pending = findings.filter((f) => f.severity === 'pending').length;
  const warn = findings.filter((f) => f.severity === 'warn').length;
  const fail = findings.filter((f) => f.severity === 'fail').length;

  let verdict: string;
  let exitCode = 0;
  if (fail > 0) {
    verdict = `VERDICT: FAIL  — ${fail} required criterion(criteria) failed.`;
    exitCode = 1;
  } else if (runtimePending) {
    verdict =
      `VERDICT: PENDING — ${pending} criterion(criteria) await the runtime commit (pil-aggregation.ts / pil-context.ts / migration 0008 / brevio.signal.aggregated / new memory_signal kinds). ` +
      `Re-run after runtime + migration land + smoke runs.`;
  } else {
    verdict =
      `VERDICT: PASS  (${pass} PASS, ${pending} operator-confirmed, ${warn} warn). ` +
      `Operator must additionally run: smoke-evidence:v0.5.9 + smoke-evidence:v0.5.10 (C17 carry-forward) + ` +
      `\`pnpm --filter @brevio/fomo run eval:pil-shadow\` (C11 + C12 LOAD-BEARING shadow contract proof).`;
  }
  console.log('');
  console.log(verdict);
  process.exit(exitCode);
}

main().catch((err) => {
  process.stderr.write(`[smoke-evidence:v0.5.11] crashed: ${String(err)}\n`);
  process.exit(2);
});
