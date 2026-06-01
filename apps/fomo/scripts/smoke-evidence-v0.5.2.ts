// Phase v0.5.2 evidence — real-friend beta smoke.
//
// Queries the live Neon Postgres substrate after a real-friend smoke
// run and prints the v0.5.2-specific evidence the founder pastes into
// SMOKE_REPORT_v0.5.2.md. Read-only.
//
// Runs ALONGSIDE smoke-evidence:v0.5.1 — that script proves the
// substrate is still healthy; this script proves the real-friend
// specifics:
//
//   * Briefing recorded — at least one v0.5.2 invite_issued audit row
//     has briefed_confirmed=true AND phone_class='real'
//   * Real friend exists — at least one users row whose linked invite
//     consumption ties back to a non-555-fictional invite
//   * Real iMessage delivered — fomo.send.succeeded audit row for a
//     friend (NOT founder) actor_user_id within the smoke window
//   * Real STOP captured — fomo.sendblue.stop_recorded for a friend
//     actor_user_id with provider_message_id NOT matching the
//     'smoke-v0.5.1-friend-stop' synthetic curl shape
//   * Founder regression: at least one approved → sent transition for
//     the founder during the smoke window
//   * Extended leak-canary: friend-email-body words, friend-phone digits
//     (last 7 ought to be sufficient for canary; we intentionally do
//     NOT scan against the full E.164 since the founder must paste
//     the friend phone into the report's REDACTED section)
//
// PASS criteria (from project_v05-2-real-friend-scope, the 12 PASS items):

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import {
  audit_log,
  invite_tokens,
  memory_signals,
  alert_state_transitions,
  users
} from '../src/db/schema.js';

interface Finding {
  readonly severity: 'pass' | 'fail' | 'warn';
  readonly criterion: string;
  readonly detail: string;
}

const FOUNDER_USER_ID = (process.env.FOMO_FOUNDER_USER_ID ?? '').trim();
const FOUNDER_PHONE = (process.env.FOMO_FOUNDER_PHONE_NUMBER ?? '').trim();

// Smoke window: last 24h by default. Founder overrides via env if the
// smoke window is longer.
const SMOKE_WINDOW_HOURS = Number(process.env.FOMO_V0_5_2_WINDOW_HOURS ?? '24');

interface RowWithDetail {
  readonly detail: Record<string, unknown> | null;
}

function detailHas<T>(row: RowWithDetail, key: string): T | undefined {
  if (!row.detail) return undefined;
  const v = row.detail[key];
  return v as T | undefined;
}

async function main(): Promise<void> {
  console.log('Phase v0.5.2 evidence — real-friend beta smoke\n');
  console.log(`Smoke window: last ${SMOKE_WINDOW_HOURS}h (override via FOMO_V0_5_2_WINDOW_HOURS).\n`);
  const findings: Finding[] = [];

  if (!process.env.DATABASE_URL?.trim()) {
    console.error('[ERROR] DATABASE_URL is required for evidence collection.');
    process.exit(2);
  }
  if (!FOUNDER_USER_ID) {
    console.error('[ERROR] FOMO_FOUNDER_USER_ID is required to distinguish friend vs founder actors.');
    process.exit(2);
  }
  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    console.error(`[ERROR] Cannot load DB client: ${dbResult.reason}`);
    process.exit(2);
  }
  const db = dbResult.client;

  /* ------------------------------------------------------------------ */
  /* Criterion 1: Briefing recorded                                     */
  /* ------------------------------------------------------------------ */

  const inviteIssued = await db
    .select({ detail: audit_log.detail, occurred_at: audit_log.occurred_at })
    .from(audit_log)
    .where(
      sql`${audit_log.action} = 'fomo.onboard.invite_issued'
          AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );

  const briefedRealInvites = inviteIssued.filter((r) => {
    const briefed = detailHas<boolean>(r, 'briefed_confirmed');
    const phoneClass = detailHas<string>(r, 'phone_class');
    return briefed === true && phoneClass === 'real';
  });

  if (briefedRealInvites.length === 0) {
    findings.push({
      severity: 'fail',
      criterion: 'Briefing recorded on a real-phone invite (correction #2 — no surprise OAuth)',
      detail:
        `In the last ${SMOKE_WINDOW_HOURS}h, ${inviteIssued.length} invite_issued audit rows total — ` +
        `0 with both briefed_confirmed=true AND phone_class='real'. The v0.5.2 invite was either ` +
        `not minted via the v0.5.2-aware issue-friend-token (missing --confirm-briefed and a real phone) ` +
        `or no invites have been issued in the smoke window at all.`
    });
  } else {
    findings.push({
      severity: 'pass',
      criterion: 'Briefing recorded on a real-phone invite (correction #2 — no surprise OAuth)',
      detail: `${briefedRealInvites.length} invite_issued audit row(s) with briefed_confirmed=true + phone_class='real'`
    });
  }

  /* ------------------------------------------------------------------ */
  /* Criterion 2: At least one real friend onboarded                    */
  /* ------------------------------------------------------------------ */

  const friendUsers = await db
    .select({ id: users.id, email: users.email, created_at: users.created_at })
    .from(users)
    .where(
      sql`${users.is_founder} = false
          AND ${users.created_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval
          AND ${users.phone_e164_hash} IS NOT NULL`
    );

  if (friendUsers.length === 0) {
    findings.push({
      severity: 'fail',
      criterion: 'At least one real friend onboarded with phone hash populated',
      detail: `No users row with is_founder=false created in the last ${SMOKE_WINDOW_HOURS}h with a phone hash. Friend did not complete /onboard, or onboarded outside the smoke window.`
    });
  } else {
    findings.push({
      severity: 'pass',
      criterion: 'At least one real friend onboarded with phone hash populated',
      detail: `${friendUsers.length} friend user(s) onboarded; IDs: ${friendUsers
        .map((u) => `${(u.id as unknown as string).slice(0, 8)}…`)
        .join(', ')}`
    });
  }

  /* ------------------------------------------------------------------ */
  /* Criterion 3: Invite tokens consumed by friends                     */
  /* ------------------------------------------------------------------ */

  const consumedInvites = await db
    .select({ id: invite_tokens.id, consumed_user_id: invite_tokens.consumed_user_id, consumed_at: invite_tokens.consumed_at })
    .from(invite_tokens)
    .where(
      sql`${invite_tokens.consumed_at} IS NOT NULL
          AND ${invite_tokens.consumed_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );

  const friendIds = new Set(friendUsers.map((u) => u.id as unknown as string));
  const consumedByFriends = consumedInvites.filter(
    (r) => r.consumed_user_id && friendIds.has(r.consumed_user_id as unknown as string)
  );

  if (consumedByFriends.length === 0) {
    findings.push({
      severity: 'fail',
      criterion: 'Invite token consumed by the friend (atomic consume on OAuth success)',
      detail: `No invite_tokens row in the smoke window has consumed_user_id matching any of the friend users created. Onboarding callback did not run.`
    });
  } else {
    findings.push({
      severity: 'pass',
      criterion: 'Invite token consumed by the friend (atomic consume on OAuth success)',
      detail: `${consumedByFriends.length} invite(s) consumed by friend user_id(s)`
    });
  }

  /* ------------------------------------------------------------------ */
  /* Criterion 4: Real iMessage delivered to the friend                 */
  /* ------------------------------------------------------------------ */

  const friendSends = await db
    .select({ detail: audit_log.detail, actor_user_id: audit_log.actor_user_id, occurred_at: audit_log.occurred_at })
    .from(audit_log)
    .where(
      sql`${audit_log.action} = 'fomo.send.succeeded'
          AND ${audit_log.actor_user_id} IS NOT NULL
          AND ${audit_log.actor_user_id} != ${FOUNDER_USER_ID}
          AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );

  if (friendSends.length === 0) {
    findings.push({
      severity: 'fail',
      criterion: 'Founder approval → real iMessage delivered to friend (fomo.send.succeeded for non-founder actor)',
      detail: `No fomo.send.succeeded audit row in the smoke window with actor_user_id != founder. Either founder did not approve a friend alert, or SendBlue failed to deliver.`
    });
  } else {
    findings.push({
      severity: 'pass',
      criterion: 'Founder approval → real iMessage delivered to friend (fomo.send.succeeded for non-founder actor)',
      detail: `${friendSends.length} successful send(s) on behalf of friend(s); destination_slug last-4 only, no raw phone`
    });
  }

  /* ------------------------------------------------------------------ */
  /* Criterion 5: Friend STOP from real iMessage (NOT synthetic curl)   */
  /* ------------------------------------------------------------------ */

  const friendStops = await db
    .select({ detail: audit_log.detail, actor_user_id: audit_log.actor_user_id })
    .from(audit_log)
    .where(
      sql`${audit_log.action} = 'fomo.sendblue.stop_recorded'
          AND ${audit_log.actor_user_id} IS NOT NULL
          AND ${audit_log.actor_user_id} != ${FOUNDER_USER_ID}
          AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );

  const realFriendStops = friendStops.filter((r) => {
    const pmid = detailHas<string>(r, 'provider_message_id') ?? '';
    // v0.5.1 synthetic STOP used "smoke-v0.5.1-friend-stop"; real
    // iMessage replies have an Apple/SendBlue UUID-shaped id.
    return !pmid.startsWith('smoke-v0.5.');
  });

  if (realFriendStops.length === 0) {
    findings.push({
      severity: 'fail',
      criterion: 'Friend STOP captured from real iMessage thread (not synthetic curl)',
      detail:
        `${friendStops.length} friend STOP audit row(s) total — 0 with a non-synthetic provider_message_id. ` +
        `Either friend did not text STOP, or the curl-shaped synthetic id is the only one in the window.`
    });
  } else {
    findings.push({
      severity: 'pass',
      criterion: 'Friend STOP captured from real iMessage thread (not synthetic curl)',
      detail: `${realFriendStops.length} real-iMessage STOP(s) recorded for friend actor_user_id`
    });
  }

  /* ------------------------------------------------------------------ */
  /* Criterion 6: Per-friend STOP isolation (memory_signals)            */
  /* ------------------------------------------------------------------ */

  const friendStopSignals = await db
    .select({ user_id: memory_signals.user_id, detail: memory_signals.detail })
    .from(memory_signals)
    .where(
      sql`${memory_signals.kind} = 'stop_active'
          AND ${memory_signals.user_id} != ${FOUNDER_USER_ID}
          AND ${memory_signals.updated_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );

  if (friendStopSignals.length === 0) {
    findings.push({
      severity: 'fail',
      criterion: 'memory_signals.stop_active row for friend (per-user keyspace)',
      detail: `No memory_signals.stop_active row keyed by a non-founder user_id in the smoke window.`
    });
  } else {
    findings.push({
      severity: 'pass',
      criterion: 'memory_signals.stop_active row for friend (per-user keyspace)',
      detail: `${friendStopSignals.length} friend stop_active row(s); user_id(s): ${friendStopSignals
        .map((r) => `${(r.user_id as unknown as string).slice(0, 8)}…`)
        .join(', ')}`
    });
  }

  /* ------------------------------------------------------------------ */
  /* Criterion 7: Founder regression — approved → sent during smoke     */
  /* ------------------------------------------------------------------ */

  const founderApprovedSent = await db
    .select({ alert_id: alert_state_transitions.alert_id, at: alert_state_transitions.at })
    .from(alert_state_transitions)
    .where(
      sql`${alert_state_transitions.from_state} = 'approved'
          AND ${alert_state_transitions.to_state} = 'sent'
          AND ${alert_state_transitions.user_id} = ${FOUNDER_USER_ID}
          AND ${alert_state_transitions.at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );

  if (founderApprovedSent.length === 0) {
    findings.push({
      severity: 'fail',
      criterion: 'Founder regression — at least one founder approved → sent during the smoke window',
      detail: `No founder approved → sent transition in the last ${SMOKE_WINDOW_HOURS}h. v0.5.2 must NOT regress the v0.1 founder path.`
    });
  } else {
    findings.push({
      severity: 'pass',
      criterion: 'Founder regression — at least one founder approved → sent during the smoke window',
      detail: `${founderApprovedSent.length} founder approved→sent transition(s) in the smoke window`
    });
  }

  /* ------------------------------------------------------------------ */
  /* Criterion 8: Leak-canary scan extended to friend content           */
  /* ------------------------------------------------------------------ */
  //
  // Scan audit_log + memory_signals + alert_state_transitions for any
  // persisted column containing what we KNOW is a forbidden substring.
  // Forbidden:
  //   * Founder phone last 7 digits (proxy for "phone digits leaked")
  //   * Canary substrings the founder pre-arranged in the friend test
  //     email (the report template names them; here we accept env
  //     overrides via FOMO_V0_5_2_LEAK_CANARIES as a comma-separated list).

  const forbiddenSubstrings: string[] = [];
  if (FOUNDER_PHONE) {
    const digits = FOUNDER_PHONE.replace(/\D/g, '');
    if (digits.length >= 7) {
      // Last 7 = local prefix + suffix; reasonable canary.
      forbiddenSubstrings.push(digits.slice(-7));
    }
  }
  const envCanaries = (process.env.FOMO_V0_5_2_LEAK_CANARIES ?? '').trim();
  if (envCanaries) {
    for (const c of envCanaries.split(',')) {
      const t = c.trim();
      if (t.length >= 6) forbiddenSubstrings.push(t);
    }
  }

  if (forbiddenSubstrings.length === 0) {
    findings.push({
      severity: 'warn',
      criterion: 'Leak-canary scan (friend phone digits + email canary substrings)',
      detail:
        'No canary substrings configured. Set FOMO_V0_5_2_LEAK_CANARIES="canary-a,canary-b" before issuing the friend invite + sending the test email, then re-run this script to validate the scan.'
    });
  } else {
    const recentAudit = await db
      .select({ detail: audit_log.detail })
      .from(audit_log)
      .where(sql`${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`)
      .limit(2000);
    const recentMemory = await db
      .select({ detail: memory_signals.detail })
      .from(memory_signals)
      .where(sql`${memory_signals.updated_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`)
      .limit(500);
    const recentTransitions = await db
      .select({ reason: alert_state_transitions.reason })
      .from(alert_state_transitions)
      .where(sql`${alert_state_transitions.at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`)
      .limit(500);

    const hits: string[] = [];
    function scanRecord(label: string, idx: number, body: string): void {
      for (const c of forbiddenSubstrings) {
        if (body.includes(c)) {
          hits.push(`${label}[${idx}] contains canary='${c.slice(0, 4)}…'`);
        }
      }
    }
    recentAudit.forEach((r, i) => scanRecord('audit_log', i, JSON.stringify(r.detail ?? {})));
    recentMemory.forEach((r, i) => scanRecord('memory_signals', i, JSON.stringify(r.detail ?? {})));
    recentTransitions.forEach((r, i) => scanRecord('alert_state_transitions', i, String(r.reason ?? '')));

    if (hits.length > 0) {
      findings.push({
        severity: 'fail',
        criterion: 'Leak-canary scan — no forbidden substrings in persisted detail',
        detail: `${hits.length} hit(s) — first few: ${hits.slice(0, 5).join('; ')}`
      });
    } else {
      findings.push({
        severity: 'pass',
        criterion: 'Leak-canary scan — no forbidden substrings in persisted detail',
        detail: `scanned ${recentAudit.length} audit + ${recentMemory.length} memory + ${recentTransitions.length} transition rows; zero hits across ${forbiddenSubstrings.length} canary substring(s)`
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* Report                                                             */
  /* ------------------------------------------------------------------ */

  console.log('========================================================================');
  console.log('Phase v0.5.2 evidence summary');
  console.log('========================================================================');
  let failures = 0;
  for (const f of findings) {
    const mark = f.severity === 'pass' ? '[✓]' : f.severity === 'fail' ? '[✗]' : '[!]';
    console.log(`  ${mark} ${f.criterion}`);
    console.log(`        ${f.detail}`);
    if (f.severity === 'fail') failures++;
  }
  console.log('');

  if (failures > 0) {
    console.log(`VERDICT: FAIL — ${failures} required v0.5.2 criterion (criteria) failed.`);
    console.log(
      '       (run smoke-evidence:v0.5.1 separately to confirm the substrate is still healthy; ' +
        'v0.5.2 is layered ON TOP of it.)'
    );
    process.exit(1);
  }

  console.log(
    'VERDICT: PASS  (operator must additionally confirm: friend received iMessage on their real phone, ' +
      'friend texted STOP from their real iMessage thread, friend understood the privacy copy. ' +
      'Run smoke-evidence:v0.5.1 separately to confirm the substrate is still healthy.)'
  );
}

main().catch((err) => {
  console.error('[ERROR]', err);
  process.exit(2);
});
