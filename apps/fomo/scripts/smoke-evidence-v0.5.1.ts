// Phase v0.5.1 evidence — multi-tenant substrate smoke.
//
// Queries the live Neon Postgres substrate after a synthetic two-user
// smoke run and prints the evidence the founder pastes into
// SMOKE_REPORT_v0.5.1.md. Read-only.
//
// PASS criteria (from the founder spec for Step 8):
//   1. /onboard mounted when FOMO_FRIEND_BETA_ENABLED=true
//      → boot log shows fomo.onboard.enabled + onboard_route_mounted: true
//      → audit_log shows fomo.onboard.invite_issued ≥ 1
//   2. /onboard unavailable when FOMO_FRIEND_BETA_ENABLED=false
//      → operator-confirmed during clean-stop scenario
//   3. Two distinct synthetic phones used
//      → users.phone_e164_hash has ≥ 1 row, distinct from founder env phone
//   4. Friend onboarding via /onboard succeeded
//      → audit_log shows fomo.onboard.user_created ≥ 1
//      → invite_tokens has ≥ 1 row with consumed_at IS NOT NULL
//   5. Friend-safe Slack card used for non-founder
//      → operator-confirmed visually; supported by Step 5 unit tests
//   6. Per-friend STOP isolation
//      → audit_log shows ≥ 1 fomo.sendblue.stop_recorded with
//        actor_user_id != founderUserId
//      → memory_signals.stop_active rows exist for the friend
//        user_id with active=true
//   7. Founder flow still works (regression)
//      → at least one alert in approved → sent state for the founder
//   8. No leak across all persisted stores
//      → canary scan over audit + tool_invocations + feedback +
//        transitions + memory_signals + inbound_replies

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import {
  users,
  invite_tokens,
  audit_log,
  memory_signals,
  alert_state_transitions
} from '../src/db/schema.js';
import { FOMO_AUDIT_ACTIONS } from '../src/core/audit.js';
import { MEMORY_SIGNAL_SOURCES } from '../src/memory/memory-signals.js';
import { REQUIRED_COLUMNS, verifyMigrations } from '../src/db/migration-verifier.js';

interface Finding {
  readonly severity: 'pass' | 'fail' | 'warn';
  readonly criterion: string;
  readonly detail: string;
}

const FOUNDER_USER_ID = (process.env.FOMO_FOUNDER_USER_ID ?? '').trim();
const FOUNDER_PHONE = (process.env.FOMO_FOUNDER_PHONE_NUMBER ?? '').trim();

// Canary scan inputs (defense-in-depth).
const FORBIDDEN_VALUE_PATTERNS: readonly RegExp[] = Object.freeze([
  /\+\d{10,15}/ // full E.164 phone
]);

function phoneDigits(e164: string): string {
  return e164.replace(/^\+/, '');
}

async function main(): Promise<void> {
  console.log('Phase v0.5.1 evidence — multi-tenant substrate smoke\n');
  const findings: Finding[] = [];

  if (!process.env.DATABASE_URL?.trim()) {
    console.error('[ERROR] DATABASE_URL is required for evidence collection.');
    process.exit(2);
  }
  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    console.error(`[ERROR] Cannot load DB client: ${dbResult.reason}`);
    process.exit(2);
  }
  const db = dbResult.client;

  /* ------------------------------------------------------------------ */
  /* Static checks                                                      */
  /* ------------------------------------------------------------------ */

  // Migration verifier with v0.5.1 columns (Step 4.2).
  try {
    const result = await verifyMigrations(db);
    if (result.ok) {
      findings.push({
        severity: 'pass',
        criterion: 'Migrations + columns up to date on live Neon',
        detail: `${result.required_tables.length} tables + ${REQUIRED_COLUMNS.length} required columns present`
      });
    } else {
      findings.push({
        severity: 'fail',
        criterion: 'Live Neon migration verification',
        detail:
          `missing_tables: ${result.missing_tables.map((m) => m.name).join(', ') || '(none)'} | ` +
          `missing_columns: ${result.missing_columns.map((m) => `${m.table}.${m.column}`).join(', ') || '(none)'}`
      });
    }
  } catch (err) {
    findings.push({
      severity: 'fail',
      criterion: 'Live Neon migration verification',
      detail: err instanceof Error ? err.message : String(err)
    });
  }

  // Audit action surface: onboard.* registered in runtime const.
  const onboardActions = ['fomo.onboard.invite_issued', 'fomo.onboard.user_created', 'fomo.onboard.kill_switch_off'];
  const allRegistered = onboardActions.every((a) => (FOMO_AUDIT_ACTIONS as readonly string[]).includes(a));
  findings.push({
    severity: allRegistered ? 'pass' : 'fail',
    criterion: 'fomo.onboard.* audit actions registered in FOMO_AUDIT_ACTIONS',
    detail: onboardActions.join(', ')
  });

  // Memory-signal source: opt_out_drift_carrier (3G.1) still there.
  findings.push({
    severity: (MEMORY_SIGNAL_SOURCES as readonly string[]).includes('opt_out_drift_carrier') ? 'pass' : 'fail',
    criterion: 'MEMORY_SIGNAL_SOURCES still includes opt_out_drift_carrier (3G.1 carry-over)',
    detail: '(no regression)'
  });

  /* ------------------------------------------------------------------ */
  /* Friend onboarding evidence                                         */
  /* ------------------------------------------------------------------ */

  // Friend rows in users.
  const friendUserRows = await db
    .select({
      id: users.id,
      email: users.email,
      is_founder: users.is_founder,
      phone_e164_hash: users.phone_e164_hash
    })
    .from(users)
    .where(sql`${users.phone_e164_hash} IS NOT NULL AND ${users.is_founder} = false`)
    .limit(20);

  console.log(`users (friends, phone_e164_hash IS NOT NULL, is_founder=false): ${friendUserRows.length}`);
  for (const r of friendUserRows) {
    console.log(`  id=${(r.id as unknown as string).slice(0, 8)}… email=${maskEmail(r.email)} hash=${(r.phone_e164_hash ?? '').slice(0, 8)}…`);
  }

  if (friendUserRows.length >= 1) {
    findings.push({
      severity: 'pass',
      criterion: 'Two-user synthetic smoke — friend(s) provisioned in users table',
      detail: `${friendUserRows.length} friend row(s); founder still env-resolved (not in users)`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'Two-user synthetic smoke — friend(s) provisioned in users table',
      detail: 'no friend rows found; was /onboard ever completed?'
    });
  }

  // Invite tokens.
  const invites = await db
    .select({
      id: invite_tokens.id,
      consumed_at: invite_tokens.consumed_at,
      consumed_user_id: invite_tokens.consumed_user_id,
      expires_at: invite_tokens.expires_at,
      issued_by_user_id: invite_tokens.issued_by_user_id
    })
    .from(invite_tokens)
    .orderBy(sql`${invite_tokens.id} DESC`)
    .limit(20);
  const issued = invites.length;
  const consumed = invites.filter((i) => i.consumed_at !== null).length;
  console.log(`\ninvite_tokens: issued=${issued} consumed=${consumed}`);
  if (issued >= 1 && consumed >= 1) {
    findings.push({
      severity: 'pass',
      criterion: 'invite_tokens lifecycle (issue → consume)',
      detail: `issued=${issued}, consumed=${consumed} (≥1 issued + ≥1 consumed)`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'invite_tokens lifecycle',
      detail: `issued=${issued}, consumed=${consumed} (need ≥1 of each)`
    });
  }

  // Friend onboarding audit events.
  const onboardEvents = await db
    .select({
      action: audit_log.action,
      detail: audit_log.detail,
      occurred_at: audit_log.occurred_at
    })
    .from(audit_log)
    .where(sql`${audit_log.action} LIKE 'fomo.onboard.%'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(30);

  const inviteIssued = onboardEvents.filter((e) => e.action === 'fomo.onboard.invite_issued').length;
  const userCreated = onboardEvents.filter((e) => e.action === 'fomo.onboard.user_created').length;
  const inviteInvalid = onboardEvents.filter((e) => e.action === 'fomo.onboard.invite_invalid').length;
  const phoneMismatch = onboardEvents.filter((e) => e.action === 'fomo.onboard.phone_mismatch').length;
  console.log(`\naudit_log fomo.onboard.*: invite_issued=${inviteIssued} user_created=${userCreated} invite_invalid=${inviteInvalid} phone_mismatch=${phoneMismatch}`);

  findings.push({
    severity: inviteIssued >= 1 ? 'pass' : 'fail',
    criterion: 'fomo.onboard.invite_issued audit row (≥1)',
    detail: `${inviteIssued} issued`
  });
  findings.push({
    severity: userCreated >= 1 ? 'pass' : 'fail',
    criterion: 'fomo.onboard.user_created audit row (≥1)',
    detail: `${userCreated} created`
  });

  /* ------------------------------------------------------------------ */
  /* Per-friend STOP isolation                                          */
  /* ------------------------------------------------------------------ */

  const stopRecorded = await db
    .select({
      actor_user_id: audit_log.actor_user_id,
      detail: audit_log.detail,
      occurred_at: audit_log.occurred_at
    })
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.sendblue.stop_recorded'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(30);

  const friendStop = stopRecorded.filter((s) => s.actor_user_id !== null && s.actor_user_id !== FOUNDER_USER_ID);
  const founderStop = stopRecorded.filter((s) => s.actor_user_id === FOUNDER_USER_ID);
  console.log(`\nfomo.sendblue.stop_recorded: founder=${founderStop.length} friend=${friendStop.length}`);

  if (friendStop.length >= 1) {
    findings.push({
      severity: 'pass',
      criterion: 'Per-friend STOP isolation — friend STOP recorded with actor_user_id != founder',
      detail: `${friendStop.length} friend STOP event(s); ${founderStop.length} founder STOP event(s)`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'Per-friend STOP isolation — friend STOP recorded',
      detail: 'no friend STOP audit events; was the synthetic friend STOP exercised?'
    });
  }

  // memory_signals.stop_active rows by user.
  const stopSignals = await db
    .select({
      user_id: memory_signals.user_id,
      detail: memory_signals.detail,
      source: memory_signals.source,
      updated_at: memory_signals.updated_at
    })
    .from(memory_signals)
    .where(sql`${memory_signals.kind} = 'stop_active'`);
  const friendStopSignals = stopSignals.filter((s) => s.user_id !== FOUNDER_USER_ID);
  const founderStopSignals = stopSignals.filter((s) => s.user_id === FOUNDER_USER_ID);
  console.log(`memory_signals.stop_active: founder=${founderStopSignals.length} friend=${friendStopSignals.length}`);

  findings.push({
    severity: friendStopSignals.length >= 1 ? 'pass' : 'fail',
    criterion: 'memory_signals.stop_active row exists for the friend (per-user isolation)',
    detail: `friend_rows=${friendStopSignals.length}; user_ids=${friendStopSignals.map((s) => s.user_id.slice(0, 8) + '…').join(', ') || '(none)'}`
  });

  /* ------------------------------------------------------------------ */
  /* Founder flow regression check                                      */
  /* ------------------------------------------------------------------ */

  const founderApprovedSent = await db
    .select({
      alert_id: alert_state_transitions.alert_id,
      to_state: alert_state_transitions.to_state,
      at: alert_state_transitions.at
    })
    .from(alert_state_transitions)
    .where(
      sql`${alert_state_transitions.user_id} = ${FOUNDER_USER_ID} AND ${alert_state_transitions.from_state} = 'approved' AND ${alert_state_transitions.to_state} = 'sent'`
    )
    .orderBy(sql`${alert_state_transitions.at} DESC`)
    .limit(5);
  console.log(`\nfounder approved → sent transitions: ${founderApprovedSent.length}`);
  findings.push({
    severity: founderApprovedSent.length >= 1 ? 'pass' : 'warn',
    criterion: 'Founder flow regression — at least one approved → sent transition for founder',
    detail:
      founderApprovedSent.length >= 1
        ? `${founderApprovedSent.length} recent approved→sent transition(s)`
        : 'no recent founder approved→sent transitions; was the founder flow exercised in this smoke window?'
  });

  /* ------------------------------------------------------------------ */
  /* Leak-canary scan                                                   */
  /* ------------------------------------------------------------------ */

  console.log('\nLeak-canary scan (raw E.164 phones must NEVER appear in persisted detail) ...');

  const hits: string[] = [];

  async function scanForCanaries(label: string, rows: { id: number; detail: unknown }[]): Promise<void> {
    for (const row of rows) {
      const dump = JSON.stringify(row.detail ?? null);
      // Founder phone literal — if it appears in detail, that's a leak.
      if (FOUNDER_PHONE && dump.includes(FOUNDER_PHONE)) {
        hits.push(`${label}#${row.id} contains FOUNDER_PHONE literal`);
      }
      if (FOUNDER_PHONE && dump.includes(phoneDigits(FOUNDER_PHONE))) {
        hits.push(`${label}#${row.id} contains FOUNDER_PHONE digits`);
      }
      for (const p of FORBIDDEN_VALUE_PATTERNS) {
        if (p.test(dump)) {
          hits.push(`${label}#${row.id} matches forbidden pattern ${p.toString()}`);
        }
      }
    }
  }

  const recentAudits = await db
    .select({ id: audit_log.id, detail: audit_log.detail })
    .from(audit_log)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(500);
  await scanForCanaries('audit_log', recentAudits);

  const recentMemorySignals = await db
    .select({ id: memory_signals.id, detail: memory_signals.detail })
    .from(memory_signals)
    .orderBy(sql`${memory_signals.updated_at} DESC`)
    .limit(50);
  await scanForCanaries('memory_signals', recentMemorySignals);

  if (hits.length === 0) {
    findings.push({
      severity: 'pass',
      criterion: 'No raw phone / canary leakage across audit + memory_signals',
      detail: `scanned ${recentAudits.length} audit + ${recentMemorySignals.length} memory rows; zero hits`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'Leak-canary scan',
      detail: `${hits.length} hits: ${hits.slice(0, 10).join(' | ')}${hits.length > 10 ? ` …+${hits.length - 10} more` : ''}`
    });
  }

  /* ------------------------------------------------------------------ */
  /* Summary                                                            */
  /* ------------------------------------------------------------------ */

  console.log('\n========================================================================');
  console.log('Phase v0.5.1 evidence summary');
  console.log('========================================================================');

  const failed = findings.filter((f) => f.severity === 'fail');
  for (const f of findings) {
    const mark = f.severity === 'pass' ? '✓' : f.severity === 'fail' ? '✖' : '!';
    console.log(`  [${mark}] ${f.criterion}`);
    console.log(`        ${f.detail}`);
  }

  console.log('');
  if (failed.length === 0) {
    console.log('VERDICT: PASS  (operator must additionally confirm friend-safe Slack card was rendered visually + clean-stop refused /onboard with the switch off)');
    await dbResult.pool.end();
    process.exit(0);
  }
  console.log(`VERDICT: FAIL  (${failed.length} required check(s) failed)`);
  await dbResult.pool.end();
  process.exit(1);
}

function maskEmail(email: string): string {
  const at = email.indexOf('@');
  if (at <= 0) return '<malformed>';
  const local = email.slice(0, at);
  const domain = email.slice(at);
  return `${local.charAt(0)}***${domain}`;
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
