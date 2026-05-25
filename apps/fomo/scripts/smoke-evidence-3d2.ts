// Phase 3D.2 evidence — queries the live Neon Postgres substrate after
// a smoke-test run and prints the evidence the founder pastes into the
// SMOKE_REPORT_3D2.md.
//
// Verifies every required Phase 3D.2 check:
//   - ≥1 alert reached `queued_for_review` (3D.1 substrate still works)
//   - ≥1 alert reached `approved` OR `rejected` (3D.2 substrate works)
//   - feedback_events: ≥1 founder_approved OR founder_rejected
//   - audit_log: `fomo.slack.interaction_received` ≥ 1 (inbound observed)
//   - audit_log: `fomo.slack.approval_captured` ≥ 1 (success path)
//   - audit_log: NO `fomo.slack.signature_invalid` rows that look like
//     legitimate misconfig (warns operator if any signature failures
//     happened — could be Slack retries from an earlier malformed run)
//   - audit_log: zero rows containing forbidden keys (no body, no
//     raw payload, no full user_id, no message text)
//   - leak-canary scan extended to feedback_events.detail and
//     alert_state_transitions.reason
//
// Optional but-strongly-recommended:
//   - `fomo.slack.approval_duplicate` ≥ 1 (proves idempotency seam
//     fired against live Postgres; the runbook walks through this)
//
// Read-only. Does not write or mutate any row.

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import {
  alerts,
  alert_state_transitions,
  audit_log,
  feedback_events
} from '../src/db/schema.js';

/* ---------------------------------------------------------------------- */
/* Leak-canary scanner                                                    */
/* ---------------------------------------------------------------------- */

const FORBIDDEN_KEYS: readonly string[] = Object.freeze([
  'body_plain',
  'body_html',
  'body_snippet',
  'attachments',
  'headers',
  'raw',
  // 3D.2-specific: a leaked raw Slack payload would carry these keys
  'payload',
  'private_metadata',
  'state'
]);

const FORBIDDEN_VALUE_PATTERNS: readonly RegExp[] = Object.freeze([
  // Long base64-url blob.
  /[A-Za-z0-9_-]{200,}/,
  // Raw header dump shape.
  /^Authentication-Results:/im,
  /^Received: from/im,
  // Full Slack user id (we only want the 4-char suffix slug)
  /U[A-Z0-9]{10,}/,
  // Slack signing-secret-looking strings (just-in-case)
  /[a-f0-9]{32,}/
]);

interface LeakHit {
  readonly source: string;
  readonly id: number | string;
  readonly reason: string;
  readonly excerpt: string;
}

function scanForLeaks(source: string, id: number | string, payload: unknown): LeakHit[] {
  if (payload === null || payload === undefined) return [];
  const hits: LeakHit[] = [];
  const seen = new WeakSet<object>();

  const walk = (node: unknown, path: string): void => {
    if (node === null || node === undefined) return;
    if (typeof node === 'string') {
      for (const re of FORBIDDEN_VALUE_PATTERNS) {
        if (re.test(node)) {
          hits.push({
            source,
            id,
            reason: `${path} matches forbidden value pattern ${re.source}`,
            excerpt: node.length > 120 ? `${node.slice(0, 120)}...` : node
          });
        }
      }
      return;
    }
    if (typeof node !== 'object') return;
    if (seen.has(node as object)) return;
    seen.add(node as object);

    if (Array.isArray(node)) {
      node.forEach((v, i) => walk(v, `${path}[${i}]`));
      return;
    }
    for (const [k, v] of Object.entries(node as Record<string, unknown>)) {
      if (FORBIDDEN_KEYS.includes(k)) {
        hits.push({
          source,
          id,
          reason: `forbidden key '${k}' present in ${path}`,
          excerpt: JSON.stringify(v).slice(0, 120)
        });
      }
      walk(v, `${path}.${k}`);
    }
  };

  walk(payload, '$');
  return hits;
}

/* ---------------------------------------------------------------------- */
/* Main                                                                   */
/* ---------------------------------------------------------------------- */

interface SmokeFinding {
  readonly label: string;
  readonly status: 'pass' | 'fail' | 'warn';
  readonly detail: string;
}

async function main(): Promise<void> {
  console.log('Phase 3D.2 evidence — querying Neon Postgres substrate\n');

  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    console.error(`[ERROR] Cannot load DB client: ${dbResult.reason}`);
    process.exit(2);
  }
  const db = dbResult.client;

  const findings: SmokeFinding[] = [];

  /* ---- alerts: 3D.1 substrate carry-forward + 3D.2 outcomes ---- */
  const alertRows = await db
    .select()
    .from(alerts)
    .orderBy(sql`${alerts.created_at} DESC`)
    .limit(50);
  console.log(`alerts: ${alertRows.length} row(s)`);
  for (const r of alertRows.slice(0, 5)) {
    console.log(`  alert_id=${r.alert_id} user=${r.user_id} message=${r.message_id} label=${r.label} score=${r.score} created_at=${r.created_at.toISOString()}`);
  }
  if (alertRows.length === 0) {
    findings.push({
      label: 'alerts table populated (3D.1 carry-forward)',
      status: 'fail',
      detail: 'No alert rows. Did the polling worker fire + rank an important message + post to Slack?'
    });
  } else {
    findings.push({
      label: 'alerts table populated (3D.1 carry-forward)',
      status: 'pass',
      detail: `${alertRows.length} alert(s) created`
    });
  }
  console.log('');

  /* ---- alert state outcomes: queued_for_review + approved/rejected ---- */
  // currentState requires a per-alert query, so we tally from transitions.
  const recentTransitions = await db
    .select()
    .from(alert_state_transitions)
    .orderBy(sql`${alert_state_transitions.at} DESC`)
    .limit(200);
  console.log(`alert_state_transitions: ${recentTransitions.length} row(s) in tail`);
  let queuedCount = 0;
  let approvedCount = 0;
  let rejectedCount = 0;
  for (const t of recentTransitions) {
    if (t.to_state === 'queued_for_review') queuedCount++;
    if (t.to_state === 'approved' && t.from_state === 'queued_for_review') approvedCount++;
    if (t.to_state === 'rejected' && t.from_state === 'queued_for_review') rejectedCount++;
  }
  console.log(
    `  transitions: queued_for_review=${queuedCount}, queued→approved=${approvedCount}, queued→rejected=${rejectedCount}`
  );
  if (queuedCount === 0) {
    findings.push({
      label: 'alert reached queued_for_review (3D.1 carry-forward)',
      status: 'fail',
      detail: 'No transitions to queued_for_review. The Slack post step did not complete.'
    });
  } else {
    findings.push({
      label: 'alert reached queued_for_review (3D.1 carry-forward)',
      status: 'pass',
      detail: `${queuedCount} transition(s)`
    });
  }
  if (approvedCount + rejectedCount === 0) {
    findings.push({
      label: 'alert reached approved OR rejected (3D.2 REQUIRED)',
      status: 'fail',
      detail: 'No transitions from queued_for_review → approved/rejected. The founder must click an Approve or Reject button in Slack.'
    });
  } else {
    findings.push({
      label: 'alert reached approved OR rejected (3D.2 REQUIRED)',
      status: 'pass',
      detail: `approved=${approvedCount}, rejected=${rejectedCount}`
    });
  }
  console.log('');

  /* ---- feedback_events: founder_approved / founder_rejected ---- */
  const feedbackRows = await db
    .select()
    .from(feedback_events)
    .where(sql`${feedback_events.kind} IN ('founder_approved', 'founder_rejected')`)
    .orderBy(sql`${feedback_events.occurred_at} DESC`)
    .limit(20);
  console.log(`feedback_events (founder_approved | founder_rejected): ${feedbackRows.length} row(s)`);
  for (const r of feedbackRows.slice(0, 5)) {
    console.log(`  id=${r.id} user=${r.user_id} kind=${r.kind} alert_id=${r.alert_id ?? '<none>'} occurred_at=${r.occurred_at.toISOString()}`);
  }
  if (feedbackRows.length === 0) {
    findings.push({
      label: 'feedback events recorded (founder_approved | founder_rejected)',
      status: 'fail',
      detail: 'No matching feedback_events rows.'
    });
  } else {
    findings.push({
      label: 'feedback events recorded (founder_approved | founder_rejected)',
      status: 'pass',
      detail: `${feedbackRows.length} event(s)`
    });
  }
  console.log('');

  /* ---- audit_log: inbound interaction received ---- */
  const interactionAudits = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.slack.interaction_received'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(50);
  console.log(`audit_log fomo.slack.interaction_received: ${interactionAudits.length} entry(ies)`);
  if (interactionAudits.length === 0) {
    findings.push({
      label: 'inbound /slack/interactivity reached the server',
      status: 'fail',
      detail: 'No interaction_received entries. Did Slack POST hit your tunnel? Check ngrok/cloudflared status + Slack app config.'
    });
  } else {
    findings.push({
      label: 'inbound /slack/interactivity reached the server',
      status: 'pass',
      detail: `${interactionAudits.length} inbound POST(s) audited`
    });
  }
  console.log('');

  /* ---- audit_log: approval_captured success path ---- */
  const capturedAudits = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.slack.approval_captured'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(20);
  console.log(`audit_log fomo.slack.approval_captured: ${capturedAudits.length} entry(ies)`);
  for (const a of capturedAudits.slice(0, 5)) {
    console.log(`  id=${a.id} at=${a.occurred_at.toISOString()} detail=${JSON.stringify(a.detail)}`);
  }
  if (capturedAudits.length === 0) {
    findings.push({
      label: 'fomo.slack.approval_captured audit written (REQUIRED)',
      status: 'fail',
      detail: 'No approval_captured entries. Either the signature failed, the channel/user check failed, the alert was unknown, or the kill switch was off.'
    });
  } else {
    findings.push({
      label: 'fomo.slack.approval_captured audit written (REQUIRED)',
      status: 'pass',
      detail: `${capturedAudits.length} capture(s)`
    });
  }
  console.log('');

  /* ---- audit_log: idempotency / duplicate (recommended) ---- */
  const dupeAudits = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.slack.approval_duplicate'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(10);
  console.log(`audit_log fomo.slack.approval_duplicate: ${dupeAudits.length} entry(ies)`);
  if (dupeAudits.length === 0) {
    findings.push({
      label: 'fomo.slack.approval_duplicate audit (≥1 recommended — idempotency proof)',
      status: 'warn',
      detail: "No duplicate-click audits. The runbook recommends clicking the SAME alert's button a second time after the first capture, to prove the first-wins / idempotent semantics fire against live Postgres."
    });
  } else {
    findings.push({
      label: 'fomo.slack.approval_duplicate audit (idempotency proof against live Postgres)',
      status: 'pass',
      detail: `${dupeAudits.length} duplicate click(s) audited — first-wins invariant holds`
    });
  }
  console.log('');

  /* ---- audit_log: signature_invalid + approval_unauthorized (warn) ---- */
  const sigFailures = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.slack.signature_invalid'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(20);
  const unauthorized = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.slack.approval_unauthorized'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(20);
  console.log(`audit_log fomo.slack.signature_invalid: ${sigFailures.length}`);
  console.log(`audit_log fomo.slack.approval_unauthorized: ${unauthorized.length}`);
  if (sigFailures.length > 0 || unauthorized.length > 0) {
    findings.push({
      label: 'no legitimate signature/auth failures during the smoke window',
      status: 'warn',
      detail: `signature_invalid=${sigFailures.length}, approval_unauthorized=${unauthorized.length}. Some failures during setup are expected (Slack retries unsigned tests, ngrok URL changes). If these correspond to your real button click, your signing secret or channel/user config is wrong.`
    });
  } else {
    findings.push({
      label: 'no signature/auth failures during the smoke window',
      status: 'pass',
      detail: '0 signature_invalid, 0 approval_unauthorized'
    });
  }
  console.log('');

  /* ---- Leak canary scan ---- */
  console.log('Scanning for leak canaries in audit_log + feedback_events.detail + alert_state_transitions.reason ...');
  const leaks: LeakHit[] = [];

  const recentAuditAll = await db
    .select()
    .from(audit_log)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(500);
  for (const e of recentAuditAll) {
    leaks.push(...scanForLeaks(`audit_log[id=${e.id}, action=${e.action}]`, e.id, e.detail));
  }
  for (const r of feedbackRows) {
    leaks.push(...scanForLeaks(`feedback_events[id=${r.id}, kind=${r.kind}]`, r.id, r.detail));
  }
  for (const t of recentTransitions) {
    leaks.push(
      ...scanForLeaks(
        `alert_state_transitions[id=${t.id}, alert=${t.alert_id}].reason`,
        t.id,
        { reason: t.reason }
      )
    );
  }

  if (leaks.length === 0) {
    console.log('  ✓ no forbidden keys or value patterns found');
    findings.push({
      label: 'No raw payload / Slack user_id / body content in audit / feedback / transitions',
      status: 'pass',
      detail: `Scanned ${recentAuditAll.length} audit + ${feedbackRows.length} feedback + ${recentTransitions.length} transition rows; zero hits.`
    });
  } else {
    console.log(`  ✖ ${leaks.length} potential leak hit(s):`);
    for (const h of leaks.slice(0, 20)) {
      console.log(`    [${h.source}] ${h.reason}`);
      console.log(`      excerpt: ${h.excerpt}`);
    }
    findings.push({
      label: 'No raw payload / Slack user_id / body content in audit / feedback / transitions',
      status: 'fail',
      detail: `${leaks.length} hit(s). First: ${leaks[0]?.reason}`
    });
  }
  console.log('');

  /* ---- Verdict ---- */
  console.log('='.repeat(72));
  console.log('Phase 3D.2 evidence summary');
  console.log('='.repeat(72));
  for (const f of findings) {
    const mark = f.status === 'pass' ? '✓' : f.status === 'warn' ? '!' : '✖';
    console.log(`  [${mark}] ${f.label}`);
    console.log(`        ${f.detail}`);
  }

  const failCount = findings.filter((f) => f.status === 'fail').length;
  const warnCount = findings.filter((f) => f.status === 'warn').length;
  console.log('');
  if (failCount === 0) {
    console.log(`VERDICT: PASS  (${warnCount} warning(s); see notes above)`);
    console.log('Phase 3E SendBlue is now unblocked.');
  } else {
    console.log(`VERDICT: FAIL  (${failCount} required check(s) failed)`);
    console.log('Do NOT start Phase 3E until every required check is green.');
  }

  if (dbResult.ok) {
    await dbResult.pool.end();
  }
  process.exit(failCount > 0 ? 1 : 0);
}

main().catch((err: unknown) => {
  console.error('Evidence script crashed:', err instanceof Error ? err.message : String(err));
  process.exit(2);
});
