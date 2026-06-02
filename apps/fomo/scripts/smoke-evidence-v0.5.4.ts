// Phase v0.5.4 evidence — second-friend cross-tenant smoke.
//
// Read-only. Queries the live Neon Postgres substrate after a
// second-friend smoke run and prints the 16 PASS criteria the founder
// pastes into SMOKE_REPORT_v0.5.4.md.
//
// v0.5.4's load-bearing assertion: cross-tenant proof. Friend B can
// complete the same chain Morris did AND Morris's + founder's state
// remain bit-for-bit untouched. Criteria 13–16 are the NEW v0.5.4
// invariants on top of the 12 v0.5.2 criteria.
//
// Required env (in addition to DATABASE_URL):
//   * FOMO_FOUNDER_USER_ID — same as v0.5.2.
//   * FOMO_V0_5_4_MORRIS_USER_ID — Morris's `users.id` UUID (the
//     v0.5.2 friend). Used to identify "the OTHER friend whose state
//     must remain untouched" vs. Friend B.
//   * FOMO_V0_5_4_WINDOW_HOURS — defaults to 24. The smoke window.
//     Morris's + founder's `stop_active.updated_at` MUST predate
//     (now − window); a row updated WITHIN the window is a regression.
//
// Optional:
//   * FOMO_V0_5_4_LEAK_CANARIES — comma-separated canary substrings
//     planted in Friend B's test email body. Same shape as v0.5.2.

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import {
  audit_log,
  memory_signals,
  alert_state_transitions,
  users
} from '../src/db/schema.js';
import { FOMO_AUDIT_ACTIONS } from '../src/core/audit.js';
import { MEMORY_SIGNAL_KINDS } from '../src/memory/memory-signals.js';

interface Finding {
  readonly severity: 'pass' | 'fail' | 'warn';
  readonly criterion: string;
  readonly detail: string;
}

const FOUNDER_USER_ID = (process.env.FOMO_FOUNDER_USER_ID ?? '').trim();
const FOUNDER_PHONE = (process.env.FOMO_FOUNDER_PHONE_NUMBER ?? '').trim();
const MORRIS_USER_ID = (process.env.FOMO_V0_5_4_MORRIS_USER_ID ?? '').trim();
const SMOKE_WINDOW_HOURS = Number(process.env.FOMO_V0_5_4_WINDOW_HOURS ?? '24');

interface RowWithDetail {
  readonly detail: Record<string, unknown> | null;
}

function detailHas<T>(row: RowWithDetail, key: string): T | undefined {
  if (!row.detail) return undefined;
  const v = row.detail[key];
  return v as T | undefined;
}

async function main(): Promise<void> {
  console.log('Phase v0.5.4 evidence — second-friend cross-tenant smoke\n');
  console.log(`Smoke window: last ${SMOKE_WINDOW_HOURS}h (override via FOMO_V0_5_4_WINDOW_HOURS).\n`);

  const findings: Finding[] = [];

  if (!process.env.DATABASE_URL?.trim()) {
    console.error('[ERROR] DATABASE_URL is required for evidence collection.');
    process.exit(2);
  }
  if (!FOUNDER_USER_ID) {
    console.error('[ERROR] FOMO_FOUNDER_USER_ID is required to distinguish friend vs founder actors.');
    process.exit(2);
  }
  if (!MORRIS_USER_ID) {
    console.error(
      '[ERROR] FOMO_V0_5_4_MORRIS_USER_ID is required. v0.5.4 must diff Friend B activity against Morris\'s pre-smoke state. ' +
        'Set this to Morris\'s users.id UUID (the v0.5.2 friend, e.g. SELECT id FROM users WHERE email=\'morrismita.101@gmail.com\').'
    );
    process.exit(2);
  }

  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    console.error(`[ERROR] Cannot load DB client: ${dbResult.reason}`);
    process.exit(2);
  }
  const db = dbResult.client;

  /* ------------------------------------------------------------------ */
  /* Static registry pins (v0.5.3 carry-forward, also gates criterion 16)*/
  /* ------------------------------------------------------------------ */

  const requiredHardeningAudits = [
    'fomo.sendblue.contact_registered',
    'fomo.sendblue.contact_registration_failed',
    'fomo.send.contact_not_registered',
    'fomo.oauth.refreshed',
    'fomo.oauth.refresh_failed',
    'fomo.db.connection_error',
    'fomo.sendblue.delivery_gap_detected'
  ];
  const auditSet = new Set<string>(FOMO_AUDIT_ACTIONS);
  const missingHardening = requiredHardeningAudits.filter((a) => !auditSet.has(a));
  const memSignalSet = new Set(MEMORY_SIGNAL_KINDS as readonly string[]);

  /* ------------------------------------------------------------------ */
  /* Criterion 1: Friend B briefed BEFORE invite mint                   */
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

  findings.push({
    severity: briefedRealInvites.length > 0 ? 'pass' : 'fail',
    criterion: 'C1: Friend B briefed BEFORE invite mint (audit briefed_confirmed=true + phone_class=real)',
    detail:
      briefedRealInvites.length > 0
        ? `${briefedRealInvites.length} briefed-real invite_issued row(s) in window`
        : `In ${SMOKE_WINDOW_HOURS}h, ${inviteIssued.length} invite_issued rows total — 0 with briefed_confirmed=true AND phone_class='real'.`
  });

  /* ------------------------------------------------------------------ */
  /* Criterion 2: Invite token bound to Friend B's real E.164           */
  /* ------------------------------------------------------------------ */
  //
  // Same row as C1 — phone_class='real' implies not in 555-fictional
  // range. Recorded separately so a failure is easy to localize.

  findings.push({
    severity: briefedRealInvites.length > 0 ? 'pass' : 'fail',
    criterion: 'C2: Invite token bound to a real (non-NANPA-fictional) E.164',
    detail:
      briefedRealInvites.length > 0
        ? `phone_class='real' confirmed on ${briefedRealInvites.length} v0.5.4 invite(s)`
        : 'No invite in window has phone_class=real. issue-friend-token refuses non-555 phone without --confirm-briefed; check the mint command was the v0.5.2-aware path.'
  });

  /* ------------------------------------------------------------------ */
  /* Criterion 3: Friend B onboarded as a NEW (non-Morris) friend user  */
  /* ------------------------------------------------------------------ */

  const friendBUsers = await db
    .select({ id: users.id, email: users.email, created_at: users.created_at })
    .from(users)
    .where(
      sql`${users.is_founder} = false
          AND ${users.id} != ${MORRIS_USER_ID}
          AND ${users.created_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval
          AND ${users.phone_e164_hash} IS NOT NULL`
    );

  const friendBIds = new Set(friendBUsers.map((u) => u.id as unknown as string));

  findings.push({
    severity: friendBUsers.length > 0 ? 'pass' : 'fail',
    criterion: 'C3: Friend B onboarded — new users row, is_founder=false, NOT Morris, phone hash populated',
    detail:
      friendBUsers.length > 0
        ? `${friendBUsers.length} Friend B user(s) onboarded in window; IDs: ${friendBUsers
            .map((u) => `${(u.id as unknown as string).slice(0, 8)}…`)
            .join(', ')}`
        : `No users row with is_founder=false created in the last ${SMOKE_WINDOW_HOURS}h that is NOT Morris. Friend B did not complete /onboard, or completed outside the window.`
  });

  /* ------------------------------------------------------------------ */
  /* Criterion 4: Friend B saw privacy copy (operator-confirmed)        */
  /* ------------------------------------------------------------------ */
  //
  // Cannot be verified mechanically — the privacy paragraph is in HTML
  // the friend reads. Marked PASS if the v0.5.x privacy copy is wired
  // (fomo.onboard.enabled fires at boot with privacy_copy_bytes > 0).

  const onboardEnabled = await db
    .select({ detail: audit_log.detail })
    .from(audit_log)
    .where(
      sql`${audit_log.action} = 'fomo.onboard.enabled'
          AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    )
    .limit(1);

  findings.push({
    severity: onboardEnabled.length > 0 ? 'pass' : 'warn',
    criterion: 'C4: Privacy copy rendered at /onboard (operator-confirmed; substrate check only)',
    detail:
      onboardEnabled.length > 0
        ? `fomo.onboard.enabled audit row found in window — privacy copy was loaded at boot. Operator must additionally confirm Friend B reported seeing it.`
        : 'No fomo.onboard.enabled audit row in window. Server may not have been booted on the v0.5.4 branch during the smoke.'
  });

  /* ------------------------------------------------------------------ */
  /* Criterion 5: Friend-safe Slack card for Friend B (no body/snippet) */
  /* ------------------------------------------------------------------ */
  //
  // Verified at v0.5.1+ by the unconditional redaction invariant — any
  // alert whose user_id !== founder gets the friend-safe shape. Here
  // we check that at least one slack.review_posted (or equivalent)
  // audit row exists for a Friend B alert.

  const slackReviewActions = ['fomo.slack.review_posted', 'fomo.slack.posted', 'fomo.slack.review_message_posted'];
  const slackReviewRows = await db
    .select({ action: audit_log.action, actor_user_id: audit_log.actor_user_id, detail: audit_log.detail })
    .from(audit_log)
    .where(
      sql`${audit_log.action} = ANY(${slackReviewActions})
          AND ${audit_log.actor_user_id} IS NOT NULL
          AND ${audit_log.actor_user_id} != ${FOUNDER_USER_ID}
          AND ${audit_log.actor_user_id} != ${MORRIS_USER_ID}
          AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );

  findings.push({
    severity: slackReviewRows.length > 0 ? 'pass' : 'warn',
    criterion: 'C5: Friend-safe Slack card posted for Friend B alert (operator-confirmed redaction)',
    detail:
      slackReviewRows.length > 0
        ? `${slackReviewRows.length} slack-review audit row(s) for Friend B actor_user_id. Operator must additionally confirm the card shape (no Snippet, footer "friend-owned (user redacted)").`
        : 'No Slack-review audit row for Friend B in window. Slack review may not have fired, OR the action name has drifted from this script\'s allowlist — verify via psql.'
  });

  /* ------------------------------------------------------------------ */
  /* Criterion 6: Founder approved in Slack                             */
  /* ------------------------------------------------------------------ */

  const approvalCaptured = await db
    .select({ action: audit_log.action, actor_user_id: audit_log.actor_user_id })
    .from(audit_log)
    .where(
      sql`${audit_log.action} = 'fomo.slack.approval_captured'
          AND ${audit_log.actor_user_id} = ${FOUNDER_USER_ID}
          AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );

  findings.push({
    severity: approvalCaptured.length > 0 ? 'pass' : 'fail',
    criterion: 'C6: Founder approved in Slack (fomo.slack.approval_captured, actor=founder)',
    detail:
      approvalCaptured.length > 0
        ? `${approvalCaptured.length} founder approval(s) captured in window`
        : 'No fomo.slack.approval_captured row with actor=founder in window. Founder did not click Approve OR Slack signed-inbound verification failed.'
  });

  /* ------------------------------------------------------------------ */
  /* Criterion 7: Real iMessage delivered to Friend B                   */
  /* ------------------------------------------------------------------ */

  const friendBSends = await db
    .select({ detail: audit_log.detail, actor_user_id: audit_log.actor_user_id })
    .from(audit_log)
    .where(
      sql`${audit_log.action} = 'fomo.send.succeeded'
          AND ${audit_log.actor_user_id} IS NOT NULL
          AND ${audit_log.actor_user_id} != ${FOUNDER_USER_ID}
          AND ${audit_log.actor_user_id} != ${MORRIS_USER_ID}
          AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );

  findings.push({
    severity: friendBSends.length > 0 ? 'pass' : 'fail',
    criterion: 'C7: Real iMessage delivered to Friend B (fomo.send.succeeded for Friend B actor_user_id)',
    detail:
      friendBSends.length > 0
        ? `${friendBSends.length} successful send(s) on behalf of Friend B; destination_slug last-4 only`
        : 'No fomo.send.succeeded row with actor_user_id != founder AND != Morris in window. Either founder did not approve a Friend B alert OR SendBlue contact-gate refused (check fomo.send.contact_not_registered).'
  });

  /* ------------------------------------------------------------------ */
  /* Criterion 8: Friend B texted STOP from real iMessage               */
  /* ------------------------------------------------------------------ */

  const friendBStops = await db
    .select({ detail: audit_log.detail, actor_user_id: audit_log.actor_user_id })
    .from(audit_log)
    .where(
      sql`${audit_log.action} = 'fomo.sendblue.stop_recorded'
          AND ${audit_log.actor_user_id} IS NOT NULL
          AND ${audit_log.actor_user_id} != ${FOUNDER_USER_ID}
          AND ${audit_log.actor_user_id} != ${MORRIS_USER_ID}
          AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );

  const realFriendBStops = friendBStops.filter((r) => {
    const pmid = detailHas<string>(r, 'provider_message_id') ?? '';
    return !pmid.startsWith('smoke-v0.5.');
  });

  findings.push({
    severity: realFriendBStops.length > 0 ? 'pass' : 'fail',
    criterion: 'C8: Friend B STOP from real iMessage thread (NOT synthetic curl)',
    detail:
      realFriendBStops.length > 0
        ? `${realFriendBStops.length} real-iMessage STOP(s) recorded for Friend B`
        : `${friendBStops.length} Friend B STOP audit row(s) — 0 with a non-synthetic provider_message_id.`
  });

  /* ------------------------------------------------------------------ */
  /* Criterion 9: Per-friend STOP isolation — Friend B's stop_active    */
  /* ------------------------------------------------------------------ */

  const friendBStopSignals = await db
    .select({ user_id: memory_signals.user_id, detail: memory_signals.detail, updated_at: memory_signals.updated_at })
    .from(memory_signals)
    .where(
      sql`${memory_signals.kind} = 'stop_active'
          AND ${memory_signals.user_id} != ${FOUNDER_USER_ID}
          AND ${memory_signals.user_id} != ${MORRIS_USER_ID}
          AND ${memory_signals.updated_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );

  findings.push({
    severity: friendBStopSignals.length > 0 ? 'pass' : 'fail',
    criterion: 'C9: memory_signals.stop_active row for Friend B (per-user keyspace, freshly written)',
    detail:
      friendBStopSignals.length > 0
        ? `${friendBStopSignals.length} Friend B stop_active row(s) updated within smoke window`
        : 'No Friend B stop_active row updated in window. STOP webhook may not have resolved to Friend B\'s user_id.'
  });

  /* ------------------------------------------------------------------ */
  /* Criterion 10: Founder regression — approved → sent during smoke    */
  /* ------------------------------------------------------------------ */

  const founderApprovedSent = await db
    .select({ alert_id: alert_state_transitions.alert_id })
    .from(alert_state_transitions)
    .where(
      sql`${alert_state_transitions.from_state} = 'approved'
          AND ${alert_state_transitions.to_state} = 'sent'
          AND ${alert_state_transitions.user_id} = ${FOUNDER_USER_ID}
          AND ${alert_state_transitions.at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );

  findings.push({
    severity: founderApprovedSent.length > 0 ? 'pass' : 'fail',
    criterion: 'C10: Founder regression — at least one founder approved → sent during the smoke window',
    detail:
      founderApprovedSent.length > 0
        ? `${founderApprovedSent.length} founder approved→sent transition(s) in window`
        : `No founder approved→sent transition in window. v0.5.4 must NOT regress the v0.1 founder path.`
  });

  /* ------------------------------------------------------------------ */
  /* Criterion 11: Leak-canary scan (extended)                          */
  /* ------------------------------------------------------------------ */

  const forbiddenSubstrings: string[] = [];
  if (FOUNDER_PHONE) {
    const digits = FOUNDER_PHONE.replace(/\D/g, '');
    if (digits.length >= 7) forbiddenSubstrings.push(digits.slice(-7));
  }
  const envCanaries = (process.env.FOMO_V0_5_4_LEAK_CANARIES ?? '').trim();
  if (envCanaries) {
    for (const c of envCanaries.split(',')) {
      const t = c.trim();
      if (t.length >= 6) forbiddenSubstrings.push(t);
    }
  }

  if (forbiddenSubstrings.length === 0) {
    findings.push({
      severity: 'warn',
      criterion: 'C11: Leak-canary scan (no forbidden substrings in persisted detail)',
      detail:
        'No canary substrings configured. Set FOMO_V0_5_4_LEAK_CANARIES="canary-a,canary-b" before issuing Friend B\'s invite + sending the test email, then re-run.'
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
        if (body.includes(c)) hits.push(`${label}[${idx}] contains canary='${c.slice(0, 4)}…'`);
      }
    }
    recentAudit.forEach((r, i) => scanRecord('audit_log', i, JSON.stringify(r.detail ?? {})));
    recentMemory.forEach((r, i) => scanRecord('memory_signals', i, JSON.stringify(r.detail ?? {})));
    recentTransitions.forEach((r, i) => scanRecord('alert_state_transitions', i, String(r.reason ?? '')));

    findings.push({
      severity: hits.length === 0 ? 'pass' : 'fail',
      criterion: 'C11: Leak-canary scan — no forbidden substrings in persisted detail',
      detail:
        hits.length === 0
          ? `scanned ${recentAudit.length} audit + ${recentMemory.length} memory + ${recentTransitions.length} transition rows; zero hits across ${forbiddenSubstrings.length} canary substring(s)`
          : `${hits.length} hit(s) — first few: ${hits.slice(0, 5).join('; ')}`
    });
  }

  /* ------------------------------------------------------------------ */
  /* Criterion 12: Friend B's users row is is_founder=false             */
  /* ------------------------------------------------------------------ */

  const friendBIdsArr = [...friendBIds];
  const friendBIsFounderRows =
    friendBIdsArr.length === 0
      ? []
      : await db
          .select({ id: users.id, is_founder: users.is_founder })
          .from(users)
          .where(sql`${users.id} = ANY(${friendBIdsArr}) AND ${users.is_founder} = true`);

  findings.push({
    severity: friendBIsFounderRows.length === 0 ? 'pass' : 'fail',
    criterion: 'C12: Friend B is_founder=false (no privilege escalation through onboard)',
    detail:
      friendBIsFounderRows.length === 0
        ? `${friendBIds.size} Friend B user(s); zero have is_founder=true`
        : `${friendBIsFounderRows.length} Friend B user(s) with is_founder=true — INVESTIGATE before merge.`
  });

  /* ====================================================================
   * NEW v0.5.4 CRITERIA — cross-tenant isolation
   * ==================================================================== */

  /* ------------------------------------------------------------------ */
  /* Criterion 13 (NEW): Morris's stop_active row UNTOUCHED              */
  /* ------------------------------------------------------------------ */

  const morrisStopRows = await db
    .select({ user_id: memory_signals.user_id, detail: memory_signals.detail, updated_at: memory_signals.updated_at })
    .from(memory_signals)
    .where(sql`${memory_signals.kind} = 'stop_active' AND ${memory_signals.user_id} = ${MORRIS_USER_ID}`);

  if (morrisStopRows.length === 0) {
    // Morris may never have had a stop_active row written if v0.5.2 STOP was undone.
    // Treat absence as a soft warning — operator confirms via baseline diff.
    findings.push({
      severity: 'warn',
      criterion: 'C13 (NEW): Morris\'s stop_active row UNTOUCHED throughout smoke window',
      detail:
        'No stop_active row found for Morris. If §0 baseline showed one row, this is a REGRESSION (criterion FAIL); ' +
        'if §0 baseline showed no row, this is OK. Operator must diff against the §0 baseline snapshot.'
    });
  } else {
    const updatedDuringWindow = morrisStopRows.filter((r) => {
      const ua = r.updated_at as unknown as Date;
      const cutoff = Date.now() - SMOKE_WINDOW_HOURS * 3600 * 1000;
      return ua && ua.getTime() >= cutoff;
    });
    findings.push({
      severity: updatedDuringWindow.length === 0 ? 'pass' : 'fail',
      criterion: 'C13 (NEW): Morris\'s stop_active row UNTOUCHED throughout smoke window',
      detail:
        updatedDuringWindow.length === 0
          ? `Morris stop_active row exists; updated_at predates smoke window (no cross-tenant write). user_id=${MORRIS_USER_ID.slice(0, 8)}…`
          : `${updatedDuringWindow.length} Morris stop_active row(s) updated within smoke window — CROSS-TENANT REGRESSION. Friend B's STOP wrote to Morris's keyspace.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* Criterion 14 (NEW): Founder's stop_active row UNTOUCHED             */
  /* ------------------------------------------------------------------ */

  const founderStopRows = await db
    .select({ user_id: memory_signals.user_id, detail: memory_signals.detail, updated_at: memory_signals.updated_at })
    .from(memory_signals)
    .where(sql`${memory_signals.kind} = 'stop_active' AND ${memory_signals.user_id} = ${FOUNDER_USER_ID}`);

  if (founderStopRows.length === 0) {
    findings.push({
      severity: 'pass',
      criterion: 'C14 (NEW): Founder\'s stop_active row UNTOUCHED throughout smoke window',
      detail:
        'No stop_active row for founder (founder never STOPped themselves) — consistent baseline; cross-tenant invariant preserved.'
    });
  } else {
    const updatedDuringWindow = founderStopRows.filter((r) => {
      const ua = r.updated_at as unknown as Date;
      const cutoff = Date.now() - SMOKE_WINDOW_HOURS * 3600 * 1000;
      return ua && ua.getTime() >= cutoff;
    });
    findings.push({
      severity: updatedDuringWindow.length === 0 ? 'pass' : 'fail',
      criterion: 'C14 (NEW): Founder\'s stop_active row UNTOUCHED throughout smoke window',
      detail:
        updatedDuringWindow.length === 0
          ? `Founder stop_active row exists; updated_at predates smoke window (no cross-tenant write).`
          : `${updatedDuringWindow.length} founder stop_active row(s) updated within smoke window — CROSS-TENANT REGRESSION. Friend B's STOP wrote to founder's keyspace.`
    });
  }

  /* ------------------------------------------------------------------ */
  /* Criterion 15 (NEW): Distinct sendblue_contact_status rows per friend*/
  /* ------------------------------------------------------------------ */

  const contactStatusRows = await db
    .select({ user_id: memory_signals.user_id, detail: memory_signals.detail, updated_at: memory_signals.updated_at })
    .from(memory_signals)
    .where(sql`${memory_signals.kind} = 'sendblue_contact_status'`);

  const morrisContactStatus = contactStatusRows.filter((r) => (r.user_id as unknown as string) === MORRIS_USER_ID);
  const friendBContactStatus = contactStatusRows.filter((r) => friendBIds.has(r.user_id as unknown as string));
  const founderContactStatus = contactStatusRows.filter((r) => (r.user_id as unknown as string) === FOUNDER_USER_ID);

  const cutoff = Date.now() - SMOKE_WINDOW_HOURS * 3600 * 1000;
  const morrisRowOverwrittenDuringWindow = morrisContactStatus.some((r) => {
    const ua = r.updated_at as unknown as Date;
    return ua && ua.getTime() >= cutoff;
  });
  const friendBRowFresh = friendBContactStatus.some((r) => {
    const ua = r.updated_at as unknown as Date;
    return ua && ua.getTime() >= cutoff;
  });

  const c15Pass =
    friendBContactStatus.length >= 1 &&
    friendBRowFresh &&
    !morrisRowOverwrittenDuringWindow &&
    // distinctness: Morris's row (if it exists) keyed to Morris; Friend B's row keyed to a Friend B id.
    morrisContactStatus.every((r) => (r.user_id as unknown as string) === MORRIS_USER_ID) &&
    friendBContactStatus.every((r) => friendBIds.has(r.user_id as unknown as string));

  findings.push({
    severity: c15Pass ? 'pass' : 'fail',
    criterion: 'C15 (NEW): Distinct sendblue_contact_status rows per friend (no overwrite, Morris\'s row untouched)',
    detail: c15Pass
      ? `Friend B contact_status row(s)=${friendBContactStatus.length} (fresh), Morris contact_status row(s)=${morrisContactStatus.length} (NOT overwritten in window), founder rows=${founderContactStatus.length}. Per-user keyspace preserved.`
      : `Friend B fresh=${friendBRowFresh}, Morris overwritten=${morrisRowOverwrittenDuringWindow}, Morris rows=${morrisContactStatus.length}, Friend B rows=${friendBContactStatus.length}. INVESTIGATE — possible cross-tenant overwrite.`
  });

  /* ------------------------------------------------------------------ */
  /* Criterion 16 (NEW): v0.5.3 hardening still functional               */
  /* ------------------------------------------------------------------ */

  // (a) All 7 v0.5.3 audit actions still in FOMO_AUDIT_ACTIONS registry.
  // (b) sendblue_contact_status still in MEMORY_SIGNAL_KINDS.
  // (c) At least one of the v0.5.3 happy-path audit actions fired during
  //     the smoke window (Friend B's onboarding writes contact_registered
  //     or contact_registration_failed — same as v0.5.3 smoke item #1).

  const onboardingHardeningFired = await db
    .select({ action: audit_log.action })
    .from(audit_log)
    .where(
      sql`${audit_log.action} IN ('fomo.sendblue.contact_registered', 'fomo.sendblue.contact_registration_failed')
          AND ${audit_log.occurred_at} > now() - (${SMOKE_WINDOW_HOURS} || ' hours')::interval`
    );

  const c16Pass =
    missingHardening.length === 0 &&
    memSignalSet.has('sendblue_contact_status') &&
    onboardingHardeningFired.length > 0;

  findings.push({
    severity: c16Pass ? 'pass' : 'fail',
    criterion: 'C16 (NEW): v0.5.3 hardening still functional (registry intact + contact lifecycle fired)',
    detail: c16Pass
      ? `7/7 hardening audits registered; sendblue_contact_status kind registered; ${onboardingHardeningFired.length} contact-lifecycle audit row(s) fired during Friend B onboarding.`
      : `missing_hardening=[${missingHardening.join(',')}], sendblue_contact_status_registered=${memSignalSet.has('sendblue_contact_status')}, contact_lifecycle_rows=${onboardingHardeningFired.length}. v0.5.3 invariants have regressed.`
  });

  /* ------------------------------------------------------------------ */
  /* Report                                                             */
  /* ------------------------------------------------------------------ */

  console.log('========================================================================');
  console.log('Phase v0.5.4 evidence summary — 16 criteria (12 v0.5.2 carry-forward + 4 NEW cross-tenant)');
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
    console.log(`VERDICT: FAIL — ${failures} required v0.5.4 criterion(criteria) failed.`);
    console.log(
      '       (also run smoke-evidence:v0.5.1 + v0.5.2 + v0.5.3 to confirm prior substrate is still healthy; v0.5.4 is layered ON TOP of them.)'
    );
    process.exit(1);
  }

  console.log(
    'VERDICT: PASS  (operator must additionally confirm: Friend B received iMessage on their real phone; ' +
      'Friend B texted STOP from their real iMessage thread; Friend B understood the privacy copy; ' +
      'Morris + founder were unaware of the smoke and their state matches the §0 baseline snapshot. ' +
      'Run smoke-evidence:v0.5.1 + v0.5.2 + v0.5.3 separately to confirm prior PASS criteria still hold.)'
  );
}

main().catch((err) => {
  console.error('[ERROR]', err);
  process.exit(2);
});
