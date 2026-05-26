// Phase 3E.2 evidence — queries the live Neon Postgres substrate after
// a smoke-test run and prints the evidence the founder pastes into
// SMOKE_REPORT_3E2.md.
//
// Verifies every required Phase 3E.2 check:
//   - ≥1 alert reached `approved` (3D.2 carry-forward; founder click)
//   - ≥1 alert reached `sent` (3E.2 LOAD-BEARING; real iMessage delivered)
//   - tool_invocations: ≥1 sendblue.send_user_message row with status=success
//   - audit_log: `fomo.send.attempted` ≥ 1 (we tried at least once)
//   - audit_log: `fomo.send.succeeded` ≥ 1 (the success path fired)
//   - audit_log: NO `fomo.send.unauthorized_destination` (founder phone
//     allowlist held; if any appear, the worker refused to text and the
//     test setup is wrong)
//   - leak-canary scan: NO full phone number, NO API key shapes, NO
//     rendered message text in audit / tool_invocations / feedback /
//     transitions
//
// Recommended-WARN (gate-passable but the runbook walks through them):
//   - ≥1 row in alert_state_transitions for approved → sent for the
//     same alert_id (the load-bearing terminal proof)
//   - No `fomo.send.failed` or `fomo.send.status_unknown` for the
//     SAME alert (a clean smoke is one alert, one transition, one send)
//   - Second cycle proves idempotency: same alert NOT re-sent (no
//     duplicate sendblue.send_user_message tool_invocations row)
//
// Read-only. Does not write or mutate any row.

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import {
  alert_state_transitions,
  audit_log,
  feedback_events,
  tool_invocations
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
  'payload',
  // 3E.2-specific: a leaked rendered-text or full-phone field would carry one of these names
  'content',
  'rendered_text',
  'phone',
  'phone_number',
  'to',
  'api_key',
  'api_secret',
  'apiKeyId',
  'apiSecretKey'
]);

// We resolve the founder phone at runtime so we can canary-scan for it
// directly. The preflight requires E.164 format like +14155551234.
const FOUNDER_PHONE = (process.env.FOMO_FOUNDER_PHONE_NUMBER ?? '').trim();
const FOUNDER_PHONE_DIGITS = FOUNDER_PHONE.replace(/^\+/, '');

const FORBIDDEN_VALUE_PATTERNS: readonly RegExp[] = Object.freeze([
  // Long base64-url blob (catches API keys, signing secrets, raw payloads).
  /[A-Za-z0-9_-]{200,}/,
  // Raw header dump shape.
  /^Authentication-Results:/im,
  /^Received: from/im,
  // Anything that looks like a 32+ hex string (signing secrets, API
  // secrets, hashes — none of these should appear in audit detail).
  /[a-f0-9]{32,}/,
  // Full E.164 phone numbers (10-15 digits with optional +). The
  // allowed form is destination_slug = last 4 digits.
  /\+\d{10,15}/,
  // SendBlue API-key-ish prefixes (defensive — these are hypothetical
  // since we don't know the exact prefix shape, but a long opaque
  // identifier in detail is suspect).
  /sk-[A-Za-z0-9_-]{20,}/
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
      // Direct-match canary: the full founder phone must never appear.
      if (FOUNDER_PHONE.length > 0 && (node.includes(FOUNDER_PHONE) || node.includes(FOUNDER_PHONE_DIGITS))) {
        hits.push({
          source,
          id,
          reason: `${path} contains the full FOMO_FOUNDER_PHONE_NUMBER (only the 4-char destination_slug is allowed)`,
          excerpt: node.length > 120 ? `${node.slice(0, 120)}...` : node
        });
      }
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
  console.log('Phase 3E.2 evidence — querying Neon Postgres substrate\n');

  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    console.error(`[ERROR] Cannot load DB client: ${dbResult.reason}`);
    process.exit(2);
  }
  const db = dbResult.client;

  const findings: SmokeFinding[] = [];

  /* ---- alert state outcomes: approved (3D.2 carry-forward) + sent (LOAD-BEARING) ---- */
  const recentTransitions = await db
    .select()
    .from(alert_state_transitions)
    .orderBy(sql`${alert_state_transitions.at} DESC`)
    .limit(200);
  console.log(`alert_state_transitions: ${recentTransitions.length} row(s) in tail`);
  let approvedCount = 0;
  let approvedToSentCount = 0;
  let approvedToFailedCount = 0;
  let approvedToUnknownCount = 0;
  const sentAlertIds = new Set<string>();
  for (const t of recentTransitions) {
    if (t.to_state === 'approved') approvedCount++;
    if (t.from_state === 'approved' && t.to_state === 'sent') {
      approvedToSentCount++;
      sentAlertIds.add(t.alert_id);
    }
    if (t.from_state === 'approved' && t.to_state === 'failed') approvedToFailedCount++;
    if (t.from_state === 'approved' && t.to_state === 'send_status_unknown') {
      approvedToUnknownCount++;
    }
  }
  console.log(
    `  transitions: approved=${approvedCount}, approved→sent=${approvedToSentCount}, ` +
      `approved→failed=${approvedToFailedCount}, approved→send_status_unknown=${approvedToUnknownCount}`
  );
  if (approvedCount === 0) {
    findings.push({
      label: 'alert reached approved (3D.2 carry-forward)',
      status: 'fail',
      detail:
        'No transitions to approved. The Slack approval click did not fire. ' +
        'Check the 3D.2 carry-forward path first; 3E.2 cannot proceed without an approved alert.'
    });
  } else {
    findings.push({
      label: 'alert reached approved (3D.2 carry-forward)',
      status: 'pass',
      detail: `${approvedCount} transition(s)`
    });
  }
  if (approvedToSentCount === 0) {
    findings.push({
      label: 'alert reached sent (3E.2 LOAD-BEARING — real iMessage delivered)',
      status: 'fail',
      detail:
        'No transitions from approved → sent. Either FOMO_SEND_ENABLED=false (the gate ' +
        'denied), the founder-phone allowlist refused the destination, the outbound-sender ' +
        'worker did not start, or SendBlue returned a non-success status. Check ' +
        'fomo.send.* audit rows below.'
    });
  } else {
    findings.push({
      label: 'alert reached sent (3E.2 LOAD-BEARING — real iMessage delivered)',
      status: 'pass',
      detail: `${approvedToSentCount} transition(s); alert_id(s): ${Array.from(sentAlertIds).slice(0, 3).join(', ')}`
    });
  }
  if (approvedToFailedCount > 0 || approvedToUnknownCount > 0) {
    findings.push({
      label: 'no approved → failed / send_status_unknown transitions during the smoke window',
      status: 'warn',
      detail:
        `approved→failed=${approvedToFailedCount}, approved→send_status_unknown=${approvedToUnknownCount}. ` +
        'A clean smoke is one alert, one transition, one send. Inspect the fomo.send.* audits ' +
        'below to understand the cause (auth, network, ambiguous response, etc.). NOTE: ' +
        'send_status_unknown is NOT auto-retried by design — the worker refuses to risk a duplicate iMessage.'
    });
  }
  console.log('');

  /* ---- tool_invocations: sendblue.send_user_message ---- */
  const sendInvocations = await db
    .select()
    .from(tool_invocations)
    .where(sql`${tool_invocations.tool_id} = 'sendblue.send_user_message'`)
    .orderBy(sql`${tool_invocations.occurred_at} DESC`)
    .limit(20);
  console.log(`tool_invocations sendblue.send_user_message: ${sendInvocations.length} row(s)`);
  let sendSuccess = 0;
  let sendFailure = 0;
  let sendDenied = 0;
  for (const r of sendInvocations) {
    if (r.status === 'success') sendSuccess++;
    if (r.status === 'failure') sendFailure++;
    if (r.status === 'denied') sendDenied++;
  }
  for (const r of sendInvocations.slice(0, 5)) {
    console.log(
      `  id=${r.id} invocation_id=${r.invocation_id} status=${r.status} policy=${r.policy_decision} latency=${r.latency_ms ?? '?'}ms`
    );
  }
  console.log(`  totals: success=${sendSuccess}, failure=${sendFailure}, denied=${sendDenied}`);
  if (sendSuccess === 0) {
    findings.push({
      label: 'sendblue.send_user_message tool_invocations: success ≥ 1',
      status: 'fail',
      detail:
        sendInvocations.length === 0
          ? 'No sendblue.send_user_message tool_invocations rows at all. The outbound-sender worker did not dispatch.'
          : `Zero success rows out of ${sendInvocations.length}. Inspect the failure/denied rows above.`
    });
  } else {
    findings.push({
      label: 'sendblue.send_user_message tool_invocations: success ≥ 1',
      status: 'pass',
      detail: `${sendSuccess} success row(s)`
    });
  }
  console.log('');

  /* ---- audit_log: fomo.send.attempted ---- */
  const attemptedAudits = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.send.attempted'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(20);
  console.log(`audit_log fomo.send.attempted: ${attemptedAudits.length} entry(ies)`);
  if (attemptedAudits.length === 0) {
    findings.push({
      label: 'fomo.send.attempted audit (REQUIRED — worker tried to send)',
      status: 'fail',
      detail:
        'No attempted audits. Either the outbound-sender worker never ran (FOMO_SEND_ENABLED off, ' +
        'or sendWiring not built), or no approved alert existed.'
    });
  } else {
    findings.push({
      label: 'fomo.send.attempted audit (REQUIRED — worker tried to send)',
      status: 'pass',
      detail: `${attemptedAudits.length} attempt(s) audited`
    });
  }
  console.log('');

  /* ---- audit_log: fomo.send.succeeded ---- */
  const succeededAudits = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.send.succeeded'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(20);
  console.log(`audit_log fomo.send.succeeded: ${succeededAudits.length} entry(ies)`);
  for (const a of succeededAudits.slice(0, 5)) {
    console.log(`  id=${a.id} at=${a.occurred_at.toISOString()} detail=${JSON.stringify(a.detail)}`);
  }
  if (succeededAudits.length === 0) {
    findings.push({
      label: 'fomo.send.succeeded audit (REQUIRED — provider confirmed delivery)',
      status: 'fail',
      detail:
        'No succeeded audits. SendBlue did not return a clear success (QUEUED/SENT/DELIVERED). ' +
        'Check fomo.send.failed and fomo.send.status_unknown rows below.'
    });
  } else {
    findings.push({
      label: 'fomo.send.succeeded audit (REQUIRED — provider confirmed delivery)',
      status: 'pass',
      detail: `${succeededAudits.length} success row(s)`
    });
  }
  console.log('');

  /* ---- audit_log: fomo.send.failed / status_unknown / unauthorized ---- */
  const failedAudits = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.send.failed'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(20);
  const unknownAudits = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.send.status_unknown'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(20);
  const unauthorizedAudits = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.send.unauthorized_destination'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(20);
  console.log(`audit_log fomo.send.failed: ${failedAudits.length}`);
  console.log(`audit_log fomo.send.status_unknown: ${unknownAudits.length}`);
  console.log(`audit_log fomo.send.unauthorized_destination: ${unauthorizedAudits.length}`);
  for (const a of failedAudits.slice(0, 3)) {
    console.log(`  [failed] id=${a.id} detail=${JSON.stringify(a.detail)}`);
  }
  for (const a of unknownAudits.slice(0, 3)) {
    console.log(`  [unknown] id=${a.id} detail=${JSON.stringify(a.detail)}`);
  }
  for (const a of unauthorizedAudits.slice(0, 3)) {
    console.log(`  [unauthorized] id=${a.id} detail=${JSON.stringify(a.detail)}`);
  }

  if (unauthorizedAudits.length > 0) {
    findings.push({
      label: 'NO fomo.send.unauthorized_destination during the smoke (LOAD-BEARING)',
      status: 'fail',
      detail:
        `${unauthorizedAudits.length} unauthorized destination(s) seen. The founder-phone ` +
        'allowlist refused to dispatch. Check FOMO_FOUNDER_PHONE_NUMBER + FOMO_FOUNDER_USER_ID; ' +
        'the user_id whose alert reached approved must match FOMO_FOUNDER_USER_ID exactly.'
    });
  } else {
    findings.push({
      label: 'NO fomo.send.unauthorized_destination during the smoke',
      status: 'pass',
      detail: '0 unauthorized-destination rows; allowlist held'
    });
  }
  if (failedAudits.length > 0 || unknownAudits.length > 0) {
    findings.push({
      label: 'no fomo.send.failed / status_unknown during the smoke window',
      status: 'warn',
      detail:
        `failed=${failedAudits.length}, status_unknown=${unknownAudits.length}. ` +
        'These do NOT block PASS by themselves — they may be from earlier troubleshooting cycles ' +
        '(e.g. before keys were configured). But the SAME alert_id should not appear in both succeeded ' +
        'and failed; if it does, investigate before merging.'
    });
  }
  console.log('');

  /* ---- audit_log: fomo.outbound.cycle_cap_reached (recommended) ---- */
  // This is a stdout log event, not an audit row, so we can't query for
  // it here. The runbook instructs the founder to grep /tmp/fomo-3e2.log
  // for the line. We surface it as a finding placeholder for the report.
  findings.push({
    label: 'fomo.outbound.cycle_cap_reached log line (RECOMMENDED — proves the cap fired)',
    status: 'warn',
    detail:
      'This is a stdout log event, not an audit row. Verify manually: ' +
      "`grep 'fomo.outbound.cycle_cap_reached' /tmp/fomo-3e2.log` should return at least one " +
      'line if FOMO_OUTBOUND_MAX_CYCLES was set. Paste the matching line into §6 of the report.'
  });

  /* ---- Leak canary scan ---- */
  console.log(
    'Scanning for leak canaries in audit_log + tool_invocations.metadata + ' +
      'feedback_events.detail + alert_state_transitions.reason ...'
  );
  if (FOUNDER_PHONE.length > 0) {
    console.log(`  (scanning for the literal FOMO_FOUNDER_PHONE_NUMBER value — must not appear anywhere)`);
  } else {
    console.log(
      '  [warn] FOMO_FOUNDER_PHONE_NUMBER is not set in this shell, so the literal-phone canary is skipped. ' +
        'The pattern-based canary still runs.'
    );
  }
  const leaks: LeakHit[] = [];

  const recentAuditAll = await db
    .select()
    .from(audit_log)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(500);
  for (const e of recentAuditAll) {
    leaks.push(...scanForLeaks(`audit_log[id=${e.id}, action=${e.action}]`, e.id, e.detail));
  }
  const recentToolInv = await db
    .select()
    .from(tool_invocations)
    .orderBy(sql`${tool_invocations.occurred_at} DESC`)
    .limit(500);
  for (const r of recentToolInv) {
    leaks.push(
      ...scanForLeaks(`tool_invocations[id=${r.id}, tool=${r.tool_id}]`, r.id, r.metadata)
    );
  }
  const recentFeedback = await db
    .select()
    .from(feedback_events)
    .orderBy(sql`${feedback_events.occurred_at} DESC`)
    .limit(200);
  for (const r of recentFeedback) {
    leaks.push(
      ...scanForLeaks(`feedback_events[id=${r.id}, kind=${r.kind}]`, r.id, r.detail)
    );
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
      label: 'No rendered text / full phone / API keys in audit / tool_invocations / feedback / transitions',
      status: 'pass',
      detail: `Scanned ${recentAuditAll.length} audit + ${recentToolInv.length} tool_invocations + ${recentFeedback.length} feedback + ${recentTransitions.length} transition rows; zero hits.`
    });
  } else {
    console.log(`  ✖ ${leaks.length} potential leak hit(s):`);
    for (const h of leaks.slice(0, 20)) {
      console.log(`    [${h.source}] ${h.reason}`);
      console.log(`      excerpt: ${h.excerpt}`);
    }
    findings.push({
      label: 'No rendered text / full phone / API keys in audit / tool_invocations / feedback / transitions',
      status: 'fail',
      detail: `${leaks.length} hit(s). First: ${leaks[0]?.reason}`
    });
  }
  console.log('');

  /* ---- Verdict ---- */
  console.log('='.repeat(72));
  console.log('Phase 3E.2 evidence summary');
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
    console.log('Phase 3F SendBlue inbound is now unblocked.');
  } else {
    console.log(`VERDICT: FAIL  (${failCount} required check(s) failed)`);
    console.log('Do NOT start Phase 3F until every required check is green.');
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
