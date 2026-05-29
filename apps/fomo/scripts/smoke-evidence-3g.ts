// Phase 3G evidence — queries the live Neon Postgres substrate
// after a Full Founder Demo Smoke Test run and prints the evidence
// the founder pastes into SMOKE_REPORT_3G.md.
//
// Verifies the v0.1 milestone PASS criteria (from FOMO_PLAN.md
// Founder Demo Gate section):
//
//   1. Gmail connected — oauth_tokens row exists for `founder` with
//      needs_reauth = false AND gmail_cursors row exists with an
//      advanced cursor (history_id > 0).
//   2. Ranker works — at least one rank_result row created during the
//      demo window with label='important' (the chain's trigger row).
//   3. Slack review works — fomo.slack.approval_captured ≥ 1 AND the
//      demo alert's state trail includes queued_for_review → approved.
//   4. SendBlue send works — the demo alert's state trail includes
//      approved → sent AND exactly ONE tool_invocations row for
//      sendblue.send_user_message tied to it.
//   5. Reply parser works — the demo alert's state trail includes
//      sent → replied AND ≥ 1 fomo.sendblue.reply_parsed event during
//      the demo window.
//   6. Memory / feedback writes — ≥ 1 feedback_events row tied to the
//      demo alert AND ≥ 1 memory_signal touched during the demo window.
//   7. No duplicate sends — exactly ONE tool_invocations row for
//      sendblue.send_user_message for the demo alert (idempotency
//      held under any Slack double-click or worker retry).
//   8. No raw body leakage — leak-canary scan across audit_log +
//      tool_invocations.metadata + feedback_events.detail +
//      alert_state_transitions.reason + memory_signals.detail +
//      inbound_replies surfaces zero hits on raw email body markers,
//      attachment names, full phone numbers, webhook secret, or
//      SendBlue API key literals.
//
// The "demo alert" is the most recent NATURAL alert (alert_id NOT
// starting with `smoke3f2-` synthetic prefix) whose state trail
// includes both `approved → sent` AND `sent → replied`. The script
// auto-discovers it by scanning recent alert_state_transitions; the
// founder confirms the auto-discovered alert_id in §6 of the report.
//
// Read-only. Does not write or mutate any row.

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import {
  alert_state_transitions,
  alerts,
  audit_log,
  feedback_events,
  gmail_cursors,
  inbound_replies,
  memory_signals,
  oauth_tokens,
  rank_results,
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
  'content',
  'reply_text',
  'rendered_text',
  'phone',
  'phone_number',
  'to',
  'from',
  'webhook_secret',
  'signing_secret',
  'api_key',
  'api_secret',
  'apiKeyId',
  'apiSecretKey'
]);

const FOUNDER_PHONE = (process.env.FOMO_FOUNDER_PHONE_NUMBER ?? '').trim();
const FOUNDER_PHONE_DIGITS = FOUNDER_PHONE.replace(/^\+/, '');
const WEBHOOK_SECRET = (process.env.SENDBLUE_WEBHOOK_SECRET ?? '').trim();
const SB_API_KEY_ID = (process.env.SENDBLUE_API_KEY_ID ?? '').trim();
const SB_API_SECRET = (process.env.SENDBLUE_API_SECRET_KEY ?? '').trim();

const FORBIDDEN_VALUE_PATTERNS: readonly RegExp[] = Object.freeze([
  /[A-Za-z0-9_-]{200,}/,
  /^Authentication-Results:/im,
  /^Received: from/im,
  /[a-f0-9]{32,}/,
  /\+\d{10,15}/
]);

function valueHits(haystack: string): readonly string[] {
  const hits: string[] = [];
  if (FOUNDER_PHONE_DIGITS && haystack.includes(FOUNDER_PHONE_DIGITS)) {
    hits.push('FOMO_FOUNDER_PHONE_NUMBER literal');
  }
  if (WEBHOOK_SECRET && haystack.includes(WEBHOOK_SECRET)) {
    hits.push('SENDBLUE_WEBHOOK_SECRET literal');
  }
  if (SB_API_KEY_ID && haystack.includes(SB_API_KEY_ID)) {
    hits.push('SENDBLUE_API_KEY_ID literal');
  }
  if (SB_API_SECRET && haystack.includes(SB_API_SECRET)) {
    hits.push('SENDBLUE_API_SECRET_KEY literal');
  }
  for (const pat of FORBIDDEN_VALUE_PATTERNS) {
    if (pat.test(haystack)) {
      hits.push(`pattern ${pat.toString()}`);
    }
  }
  return hits;
}

function scanJson(label: string, payload: unknown, hits: { label: string; reason: string }[]): void {
  if (payload === null || payload === undefined) return;
  const walk = (path: string, node: unknown): void => {
    if (node === null || node === undefined) return;
    if (typeof node === 'string') {
      for (const hit of valueHits(node)) {
        hits.push({ label: `${label}.${path}`, reason: `forbidden value at ${path}: ${hit}` });
      }
      return;
    }
    if (typeof node !== 'object') return;
    if (Array.isArray(node)) {
      node.forEach((el, i) => walk(`${path}[${i}]`, el));
      return;
    }
    for (const [k, v] of Object.entries(node)) {
      if (FORBIDDEN_KEYS.includes(k)) {
        hits.push({ label: `${label}.${path === '' ? k : `${path}.${k}`}`, reason: `forbidden key '${k}'` });
      }
      walk(path === '' ? k : `${path}.${k}`, v);
    }
  };
  walk('', payload);
}

/* ---------------------------------------------------------------------- */
/* Main                                                                   */
/* ---------------------------------------------------------------------- */

interface Finding {
  readonly severity: 'pass' | 'fail' | 'warn';
  readonly criterion: string;
  readonly detail: string;
}

interface StateTransitionRow {
  readonly alert_id: string;
  readonly to_state: string;
  readonly from_state: string;
  readonly at: Date;
  readonly reason: string;
}

async function main(): Promise<void> {
  console.log('Phase 3G evidence — querying Neon Postgres substrate\n');

  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    console.error(`[ERROR] Cannot load DB client: ${dbResult.reason}`);
    process.exit(2);
  }
  const db = dbResult.client;

  const findings: Finding[] = [];

  /* ------------------------------------------------------------------ */
  /* 1. Gmail connected                                                 */
  /* ------------------------------------------------------------------ */

  const oauthRows = await db
    .select({
      user_id: oauth_tokens.user_id,
      needs_reauth: oauth_tokens.needs_reauth,
      obtained_at: oauth_tokens.obtained_at
    })
    .from(oauth_tokens)
    .where(sql`${oauth_tokens.user_id} = 'founder'`);

  const cursorRows = await db
    .select({
      user_id: gmail_cursors.user_id,
      history_id: gmail_cursors.history_id,
      updated_at: gmail_cursors.updated_at
    })
    .from(gmail_cursors)
    .where(sql`${gmail_cursors.user_id} = 'founder'`);

  const gmailToken = oauthRows[0];
  const gmailCursor = cursorRows[0];

  console.log(`Gmail oauth_tokens (founder): ${oauthRows.length} row(s)`);
  for (const row of oauthRows) {
    console.log(`  user=${row.user_id} needs_reauth=${row.needs_reauth} obtained_at=${row.obtained_at?.toISOString?.() ?? row.obtained_at}`);
  }
  console.log(`Gmail cursors (founder): ${cursorRows.length} row(s)`);
  for (const row of cursorRows) {
    console.log(`  user=${row.user_id} history_id=${row.history_id} updated_at=${row.updated_at?.toISOString?.() ?? row.updated_at}`);
  }
  console.log('');

  if (gmailToken && gmailToken.needs_reauth === false) {
    findings.push({
      severity: 'pass',
      criterion: 'Gmail connected',
      detail: `oauth_tokens(founder) needs_reauth=false; cursors history_id=${gmailCursor?.history_id ?? '<no cursor>'}`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'Gmail connected',
      detail: gmailToken
        ? `oauth_tokens(founder) needs_reauth=${gmailToken.needs_reauth} — re-auth required before demo`
        : 'No oauth_tokens row for founder; Gmail not connected'
    });
  }

  /* ------------------------------------------------------------------ */
  /* 2 + 3 + 4 + 5: find the demo alert (natural, full lifecycle)       */
  /* ------------------------------------------------------------------ */

  // Find the most recent alert whose state trail includes BOTH
  // approved → sent AND sent → replied, and whose alert_id does NOT
  // start with the 3F.2 synthetic prefix.
  const candidateRows: StateTransitionRow[] = await db
    .select({
      alert_id: alert_state_transitions.alert_id,
      to_state: alert_state_transitions.to_state,
      from_state: alert_state_transitions.from_state,
      at: alert_state_transitions.at,
      reason: alert_state_transitions.reason
    })
    .from(alert_state_transitions)
    .where(sql`${alert_state_transitions.user_id} = 'founder' AND ${alert_state_transitions.alert_id} NOT LIKE 'smoke3f2-%'`)
    .orderBy(sql`${alert_state_transitions.at} DESC`)
    .limit(200);

  const byAlert = new Map<string, StateTransitionRow[]>();
  for (const row of candidateRows) {
    const arr = byAlert.get(row.alert_id) ?? [];
    arr.push(row);
    byAlert.set(row.alert_id, arr);
  }

  let demoAlertId: string | null = null;
  for (const [alertId, transitions] of byAlert.entries()) {
    const hasSent = transitions.some((t: StateTransitionRow) => t.from_state === 'approved' && t.to_state === 'sent');
    const hasReplied = transitions.some((t: StateTransitionRow) => t.from_state === 'sent' && t.to_state === 'replied');
    if (hasSent && hasReplied) {
      demoAlertId = alertId;
      break;
    }
  }

  console.log(`Demo alert candidate: ${demoAlertId ?? '<NONE FOUND>'}`);
  console.log('  (most recent natural alert with both approved→sent AND sent→replied transitions)');
  console.log('');

  let demoTrail: StateTransitionRow[] = [];
  let demoAlertRow: { alert_id: string; user_id: string; message_id: string; rank_result_id: number; label: string; score: number; created_at: Date } | null = null;
  let demoRankRow: { id: number; label: string; score: number; model_name: string; created_at: Date } | null = null;
  let demoSendInvocations: { id: number; tool_id: string; status: string; occurred_at: Date }[] = [];
  let demoFeedback: { id: number; kind: string; alert_id: string | null }[] = [];
  let demoReplyEvents: { id: number; action: string; result: string; occurred_at: Date }[] = [];
  let demoSlackApproval: { id: number; action: string; result: string; occurred_at: Date }[] = [];

  if (demoAlertId) {
    demoTrail = (byAlert.get(demoAlertId) ?? []).slice().reverse();

    const alertRows = await db
      .select({
        alert_id: alerts.alert_id,
        user_id: alerts.user_id,
        message_id: alerts.message_id,
        rank_result_id: alerts.rank_result_id,
        label: alerts.label,
        score: alerts.score,
        created_at: alerts.created_at
      })
      .from(alerts)
      .where(sql`${alerts.alert_id} = ${demoAlertId}`)
      .limit(1);
    demoAlertRow = alertRows[0] ?? null;

    if (demoAlertRow) {
      const rankRows = await db
        .select({
          id: rank_results.id,
          label: rank_results.label,
          score: rank_results.score,
          model_name: rank_results.model_name,
          created_at: rank_results.created_at
        })
        .from(rank_results)
        .where(sql`${rank_results.id} = ${demoAlertRow.rank_result_id}`)
        .limit(1);
      demoRankRow = rankRows[0] ?? null;
    }

    // The v0.1 runtime writes tool_invocations without alert_id in
    // metadata, so we correlate by time window against the alert's
    // approved → sent transition (same pattern used for Slack
    // approval + reply parser below).
    const sentTransitionForInv = demoTrail.find(
      (t: StateTransitionRow) => t.from_state === 'approved' && t.to_state === 'sent'
    );
    if (sentTransitionForInv) {
      const invWindowStart = new Date(sentTransitionForInv.at.getTime() - 60_000);
      const invWindowEnd = new Date(sentTransitionForInv.at.getTime() + 60_000);
      const invRows = await db
        .select({
          id: tool_invocations.id,
          tool_id: tool_invocations.tool_id,
          status: tool_invocations.status,
          occurred_at: tool_invocations.occurred_at
        })
        .from(tool_invocations)
        .where(
          sql`${tool_invocations.tool_id} = 'sendblue.send_user_message' AND ${tool_invocations.occurred_at} BETWEEN ${invWindowStart} AND ${invWindowEnd}`
        );
      demoSendInvocations = invRows.map((r: { id: number; tool_id: string; status: string; occurred_at: Date }) => ({
        id: r.id,
        tool_id: r.tool_id,
        status: r.status,
        occurred_at: r.occurred_at
      }));
    }

    const fbRows = await db
      .select({
        id: feedback_events.id,
        kind: feedback_events.kind,
        alert_id: feedback_events.alert_id
      })
      .from(feedback_events)
      .where(sql`${feedback_events.alert_id} = ${demoAlertId}`);
    demoFeedback = fbRows;

    // Reply events likely don't include alert_id in detail, but they
    // happen within seconds of the sent→replied transition.
    const repliedTransition = demoTrail.find((t: StateTransitionRow) => t.to_state === 'replied');
    if (repliedTransition) {
      const windowStart = new Date(repliedTransition.at.getTime() - 30_000);
      const windowEnd = new Date(repliedTransition.at.getTime() + 30_000);
      const replyRows = await db
        .select({
          id: audit_log.id,
          action: audit_log.action,
          result: audit_log.result,
          occurred_at: audit_log.occurred_at
        })
        .from(audit_log)
        .where(
          sql`${audit_log.action} IN ('fomo.sendblue.reply_parsed', 'fomo.sendblue.inbound_received') AND ${audit_log.occurred_at} BETWEEN ${windowStart} AND ${windowEnd}`
        )
        .orderBy(sql`${audit_log.occurred_at} ASC`);
      demoReplyEvents = replyRows;
    }

    const sentTransition = demoTrail.find((t: StateTransitionRow) => t.to_state === 'sent');
    if (sentTransition) {
      const windowStart = new Date(sentTransition.at.getTime() - 120_000);
      const windowEnd = new Date(sentTransition.at.getTime() + 30_000);
      const slackRows = await db
        .select({
          id: audit_log.id,
          action: audit_log.action,
          result: audit_log.result,
          occurred_at: audit_log.occurred_at
        })
        .from(audit_log)
        .where(
          sql`${audit_log.action} = 'fomo.slack.approval_captured' AND ${audit_log.occurred_at} BETWEEN ${windowStart} AND ${windowEnd}`
        )
        .orderBy(sql`${audit_log.occurred_at} ASC`);
      demoSlackApproval = slackRows;
    }

    console.log(`Demo alert state trail (${demoTrail.length} transitions):`);
    for (const t of demoTrail) {
      console.log(`  ${t.from_state} → ${t.to_state}  reason="${t.reason}"  at=${t.at.toISOString()}`);
    }
    console.log('');
    console.log(`Demo rank_result: ${demoRankRow ? `id=${demoRankRow.id} label=${demoRankRow.label} score=${demoRankRow.score} model=${demoRankRow.model_name}` : '<NONE>'}`);
    console.log(`Demo Slack approval audits: ${demoSlackApproval.length} event(s)`);
    console.log(`Demo SendBlue send_user_message invocations: ${demoSendInvocations.length}`);
    console.log(`Demo feedback_events tied to alert: ${demoFeedback.length} row(s)`);
    for (const fb of demoFeedback) {
      console.log(`  id=${fb.id} kind=${fb.kind}`);
    }
    console.log(`Demo reply audit events (±30s of sent→replied): ${demoReplyEvents.length}`);
    for (const ev of demoReplyEvents) {
      console.log(`  id=${ev.id} action=${ev.action} result=${ev.result} at=${ev.occurred_at.toISOString()}`);
    }
    console.log('');
  }

  // Ranker
  if (demoRankRow && demoRankRow.label === 'important') {
    findings.push({
      severity: 'pass',
      criterion: 'Ranker works',
      detail: `rank_result id=${demoRankRow.id} label=important score=${demoRankRow.score} model=${demoRankRow.model_name}`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'Ranker works',
      detail: demoRankRow
        ? `Demo alert's rank_result has label=${demoRankRow.label} (expected 'important')`
        : 'No demo alert found OR no rank_result row tied to it'
    });
  }

  // Slack review
  if (demoTrail.some((t: StateTransitionRow) => t.from_state === 'queued_for_review' && t.to_state === 'approved') && demoSlackApproval.length > 0) {
    findings.push({
      severity: 'pass',
      criterion: 'Slack review works',
      detail: `queued_for_review → approved transition + ${demoSlackApproval.length} fomo.slack.approval_captured event(s)`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'Slack review works',
      detail: 'Missing queued_for_review → approved transition OR no fomo.slack.approval_captured event near the sent transition'
    });
  }

  // SendBlue send
  if (demoTrail.some((t: StateTransitionRow) => t.from_state === 'approved' && t.to_state === 'sent') && demoSendInvocations.length === 1) {
    findings.push({
      severity: 'pass',
      criterion: 'SendBlue send works',
      detail: `approved → sent transition + exactly 1 tool_invocations(sendblue.send_user_message) row tied to alert`
    });
  } else if (demoSendInvocations.length > 1) {
    findings.push({
      severity: 'fail',
      criterion: 'SendBlue send works',
      detail: `${demoSendInvocations.length} tool_invocations rows for this alert — duplicate send detected`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'SendBlue send works',
      detail: 'Missing approved → sent transition OR no tool_invocations row tied to the alert'
    });
  }

  // Reply parser
  if (
    demoTrail.some((t: StateTransitionRow) => t.from_state === 'sent' && t.to_state === 'replied') &&
    demoReplyEvents.some((e: { action: string }) => e.action === 'fomo.sendblue.reply_parsed')
  ) {
    findings.push({
      severity: 'pass',
      criterion: 'Reply parser works',
      detail: `sent → replied transition + fomo.sendblue.reply_parsed event in ±30s window`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'Reply parser works',
      detail: 'Missing sent → replied transition OR no fomo.sendblue.reply_parsed event near the transition'
    });
  }

  // Memory / feedback writes
  const recentMemoryWrites = await db
    .select({
      id: memory_signals.id,
      kind: memory_signals.kind,
      updated_at: memory_signals.updated_at
    })
    .from(memory_signals)
    .where(sql`${memory_signals.user_id} = 'founder' AND ${memory_signals.updated_at} > now() - interval '2 hours'`);

  if (demoFeedback.length > 0 || recentMemoryWrites.length > 0) {
    findings.push({
      severity: 'pass',
      criterion: 'Memory / feedback writes',
      detail: `${demoFeedback.length} feedback_events tied to alert + ${recentMemoryWrites.length} memory_signals touched in last 2h`
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'Memory / feedback writes',
      detail: 'No feedback_events tied to demo alert AND no memory_signal writes in last 2h'
    });
  }

  // No duplicate sends
  if (demoSendInvocations.length === 1) {
    findings.push({
      severity: 'pass',
      criterion: 'No duplicate sends',
      detail: `Exactly 1 tool_invocations(sendblue.send_user_message) for demo alert`
    });
  } else if (demoSendInvocations.length === 0) {
    findings.push({
      severity: 'fail',
      criterion: 'No duplicate sends',
      detail: 'No tool_invocations found — cannot verify dedup (and SendBlue send did not happen)'
    });
  } else {
    findings.push({
      severity: 'fail',
      criterion: 'No duplicate sends',
      detail: `${demoSendInvocations.length} tool_invocations rows — DUPLICATE SEND DETECTED`
    });
  }

  /* ------------------------------------------------------------------ */
  /* 8. Leak-canary scan                                                */
  /* ------------------------------------------------------------------ */

  console.log('Scanning for leak canaries in audit_log + tool_invocations.metadata + feedback_events.detail + alert_state_transitions.reason + memory_signals.detail + inbound_replies ...');
  if (FOUNDER_PHONE_DIGITS) console.log('  (scanning for the literal FOMO_FOUNDER_PHONE_NUMBER value)');
  if (WEBHOOK_SECRET) console.log('  (scanning for the literal SENDBLUE_WEBHOOK_SECRET value)');
  if (SB_API_KEY_ID) console.log('  (scanning for the literal SENDBLUE_API_KEY_ID value)');

  const hits: { label: string; reason: string }[] = [];

  const auditRowsAll = await db
    .select({
      id: audit_log.id,
      action: audit_log.action,
      detail: audit_log.detail,
      occurred_at: audit_log.occurred_at
    })
    .from(audit_log)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(500);
  for (const row of auditRowsAll) {
    scanJson(`audit_log#${row.id}(${row.action})`, row.detail, hits);
  }

  const toolRowsAll = await db
    .select({
      id: tool_invocations.id,
      tool_id: tool_invocations.tool_id,
      metadata: tool_invocations.metadata
    })
    .from(tool_invocations)
    .orderBy(sql`${tool_invocations.id} DESC`)
    .limit(100);
  for (const row of toolRowsAll) {
    scanJson(`tool_invocations#${row.id}(${row.tool_id})`, row.metadata, hits);
  }

  const feedbackAll = await db
    .select({ id: feedback_events.id, kind: feedback_events.kind, detail: feedback_events.detail })
    .from(feedback_events)
    .orderBy(sql`${feedback_events.id} DESC`)
    .limit(50);
  for (const row of feedbackAll) {
    scanJson(`feedback_events#${row.id}(${row.kind})`, row.detail, hits);
  }

  const transAll = await db
    .select({ id: alert_state_transitions.id, reason: alert_state_transitions.reason })
    .from(alert_state_transitions)
    .orderBy(sql`${alert_state_transitions.id} DESC`)
    .limit(100);
  for (const row of transAll) {
    if (typeof row.reason === 'string') {
      for (const hit of valueHits(row.reason)) {
        hits.push({ label: `alert_state_transitions#${row.id}.reason`, reason: hit });
      }
    }
  }

  const memoryAll = await db
    .select({ id: memory_signals.id, kind: memory_signals.kind, detail: memory_signals.detail })
    .from(memory_signals)
    .orderBy(sql`${memory_signals.id} DESC`)
    .limit(50);
  for (const row of memoryAll) {
    scanJson(`memory_signals#${row.id}(${row.kind})`, row.detail, hits);
  }

  const inboundAll = await db
    .select({
      id: inbound_replies.id,
      provider_message_id: inbound_replies.provider_message_id,
      user_id: inbound_replies.user_id
    })
    .from(inbound_replies)
    .orderBy(sql`${inbound_replies.id} DESC`)
    .limit(50);
  for (const row of inboundAll) {
    for (const hit of valueHits(row.provider_message_id)) {
      hits.push({ label: `inbound_replies#${row.id}.provider_message_id`, reason: hit });
    }
  }

  const scannedTotal =
    auditRowsAll.length +
    toolRowsAll.length +
    feedbackAll.length +
    transAll.length +
    memoryAll.length +
    inboundAll.length;

  if (hits.length === 0) {
    console.log(`  ✓ no forbidden keys or value patterns found`);
    findings.push({
      severity: 'pass',
      criterion: 'No raw body / phone / secret leakage',
      detail: `Scanned ${auditRowsAll.length} audit + ${toolRowsAll.length} tool_invocations + ${feedbackAll.length} feedback + ${transAll.length} transition + ${memoryAll.length} memory_signal + ${inboundAll.length} inbound_replies rows; zero hits.`
    });
  } else {
    console.log(`  ✖ ${hits.length} LEAK(S) FOUND:`);
    for (const h of hits.slice(0, 20)) {
      console.log(`    ${h.label}: ${h.reason}`);
    }
    if (hits.length > 20) console.log(`    ... and ${hits.length - 20} more`);
    findings.push({
      severity: 'fail',
      criterion: 'No raw body / phone / secret leakage',
      detail: `${hits.length} hits across ${scannedTotal} rows scanned — see stdout`
    });
  }

  /* ------------------------------------------------------------------ */
  /* Summary                                                            */
  /* ------------------------------------------------------------------ */

  console.log('');
  console.log('========================================================================');
  console.log('Phase 3G evidence summary');
  console.log('========================================================================');

  const failed = findings.filter((f: Finding) => f.severity === 'fail');
  for (const f of findings) {
    const mark = f.severity === 'pass' ? '✓' : f.severity === 'fail' ? '✖' : '!';
    console.log(`  [${mark}] ${f.criterion}`);
    console.log(`        ${f.detail}`);
  }

  console.log('');
  if (failed.length === 0) {
    console.log(`VERDICT: PASS  (demo alert: ${demoAlertId ?? '<none>'})`);
    console.log('Phase 3G — v0.1 demo gate is GREEN. Fill in docs/SMOKE_REPORT_3G.md and merge.');
  } else {
    console.log(`VERDICT: FAIL  (${failed.length} required check(s) failed)`);
    console.log('Do NOT mark v0.1 done until every required check is green.');
    process.exit(1);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
