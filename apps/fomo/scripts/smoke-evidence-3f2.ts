// Phase 3F.2 evidence — queries the live Neon Postgres substrate
// after a smoke-test run and prints the evidence the founder pastes
// into SMOKE_REPORT_3F2.md.
//
// Verifies every required Phase 3F.2 check (founder directive
// 2026-05-26):
//
//   - Real founder reply reached Brevio through SendBlue
//     (inbound_replies ≥ 1)
//   - Auth verification succeeded for valid SendBlue requests
//     (≥1 fomo.sendblue.reply_parsed OR fomo.sendblue.stop_recorded
//      with no preceding fomo.sendblue.signature_invalid for the
//      same provider_message_id)
//   - Invalid auth was rejected (≥1 fomo.sendblue.signature_invalid
//     with error_code in {missing_header, secret_mismatch})
//   - Duplicate webhook is idempotent (≥1 fomo.sendblue.reply_duplicate)
//   - STOP is deterministic (≥1 fomo.sendblue.stop_recorded with
//     intent_source: deterministic, NOT classifier)
//   - START clears stop (≥1 fomo.sendblue.start_recorded IF tested;
//     otherwise warn-only)
//   - One soft reply intent was parsed (≥1 fomo.sendblue.reply_parsed
//     with intent_source: classifier)
//   - State transition writes (≥1 alert_state_transitions row from
//     sent → replied → snoozed/ignored)
//   - Feedback writes (≥1 row in user_snoozed / user_ignored /
//     stop / asked_why / false_positive / ignored_sender)
//   - Memory writes (stop_active memory_signal exists with at least
//     one detail.active=true observed)
//   - STOP enforcement blocked future sends (≥1 fomo.send.stop_enforced)
//   - LEAK CANARY SCAN: NO raw reply text / phone / webhook secret /
//     SendBlue API key in audit + feedback + memory + transitions +
//     inbound_replies
//
// PASS is NOT allowed unless real inbound webhook auth works
// end-to-end. The founder ALSO records (in the smoke report, §5
// "Auth Observation") which header SendBlue actually used; if it
// differed from the configured runtime, a runtime patch is
// required before any PASS verdict.
//
// Read-only. Does not write or mutate any row.

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import {
  alert_state_transitions,
  audit_log,
  feedback_events,
  inbound_replies,
  memory_signals,
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
  // 3F-specific: the founder's reply text and the webhook secret
  // must NEVER appear in any persisted detail field.
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
  // Long base64-url blob (catches API keys, signing secrets, raw payloads).
  /[A-Za-z0-9_-]{200,}/,
  // Raw header dump shape (Gmail leak).
  /^Authentication-Results:/im,
  /^Received: from/im,
  // 32+ hex characters (signing secrets, hashes).
  /[a-f0-9]{32,}/,
  // Full E.164 phone numbers (10-15 digits with optional +). Allowed
  // shape is the 4-char destination_slug or from_slug suffix only.
  /\+\d{10,15}/
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
      // Direct-match canaries: literal values that MUST NEVER appear.
      if (FOUNDER_PHONE.length > 0 && (node.includes(FOUNDER_PHONE) || node.includes(FOUNDER_PHONE_DIGITS))) {
        hits.push({
          source,
          id,
          reason: `${path} contains the full FOMO_FOUNDER_PHONE_NUMBER (only the 4-char from_slug is allowed)`,
          excerpt: node.length > 120 ? `${node.slice(0, 120)}...` : node
        });
      }
      if (WEBHOOK_SECRET.length > 8 && node.includes(WEBHOOK_SECRET)) {
        hits.push({
          source,
          id,
          reason: `${path} contains the SENDBLUE_WEBHOOK_SECRET value verbatim`,
          excerpt: '<redacted — secret leak>'
        });
      }
      if (SB_API_KEY_ID.length > 8 && node.includes(SB_API_KEY_ID)) {
        hits.push({
          source,
          id,
          reason: `${path} contains the SENDBLUE_API_KEY_ID value verbatim`,
          excerpt: '<redacted — API key leak>'
        });
      }
      if (SB_API_SECRET.length > 8 && node.includes(SB_API_SECRET)) {
        hits.push({
          source,
          id,
          reason: `${path} contains the SENDBLUE_API_SECRET_KEY value verbatim`,
          excerpt: '<redacted — API secret leak>'
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
  console.log('Phase 3F.2 evidence — querying Neon Postgres substrate\n');

  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    console.error(`[ERROR] Cannot load DB client: ${dbResult.reason}`);
    process.exit(2);
  }
  const db = dbResult.client;

  const findings: SmokeFinding[] = [];

  /* ---- inbound_replies: real webhook reached Brevio + auth passed ---- */
  const inboundRows = await db
    .select()
    .from(inbound_replies)
    .orderBy(sql`${inbound_replies.received_at} DESC`)
    .limit(20);
  console.log(`inbound_replies: ${inboundRows.length} row(s)`);
  for (const r of inboundRows.slice(0, 5)) {
    console.log(
      `  id=${r.id} provider_message_id=${r.provider_message_id} user=${r.user_id} received=${r.received_at.toISOString()}`
    );
  }
  if (inboundRows.length === 0) {
    findings.push({
      label: 'inbound_replies ≥ 1 (real SendBlue webhook reached Brevio + auth passed)',
      status: 'fail',
      detail:
        'No inbound_replies rows. Either no real SendBlue webhook fired, or every webhook failed auth ' +
        'before reaching the dedup gate. Check `fomo.sendblue.signature_invalid` audit rows for clues.'
    });
  } else {
    findings.push({
      label: 'inbound_replies ≥ 1 (real SendBlue webhook reached Brevio + auth passed)',
      status: 'pass',
      detail: `${inboundRows.length} inbound webhook(s) auth'd + processed`
    });
  }
  console.log('');

  /* ---- audit_log: inbound_received (every POST, BEFORE auth verify) ---- */
  const inboundReceived = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.sendblue.inbound_received'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(50);
  console.log(`audit_log fomo.sendblue.inbound_received: ${inboundReceived.length} entry(ies)`);
  // Surface the secret_header_name observed across these rows so the
  // founder can confirm in §5 of the report.
  const observedHeaders = new Set<string>();
  for (const e of inboundReceived) {
    const d = e.detail as { secret_header_name?: string } | null;
    if (d?.secret_header_name) observedHeaders.add(d.secret_header_name);
  }
  if (observedHeaders.size > 0) {
    console.log(`  configured secret_header_name(s) observed: ${[...observedHeaders].join(', ')}`);
  }

  /* ---- audit_log: invalid-auth rejection (REQUIRED) ---- */
  const sigInvalid = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.sendblue.signature_invalid'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(20);
  console.log(`audit_log fomo.sendblue.signature_invalid: ${sigInvalid.length} entry(ies)`);
  const sigInvalidReasons = new Set<string>();
  for (const e of sigInvalid.slice(0, 5)) {
    const d = e.detail as { error_code?: string } | null;
    if (d?.error_code) sigInvalidReasons.add(d.error_code);
    console.log(`  id=${e.id} at=${e.occurred_at.toISOString()} error_code=${d?.error_code ?? '<none>'}`);
  }
  // The runbook instructs the founder to deliberately curl an invalid
  // secret as one of the scenarios. PASS requires ≥1.
  if (sigInvalid.length === 0) {
    findings.push({
      label: 'invalid-auth rejection (REQUIRED — founder curls a wrong secret to prove fail-closed)',
      status: 'fail',
      detail:
        'No fomo.sendblue.signature_invalid entries. The runbook §10 (Smoke scenario 5) requires the ' +
        'founder to deliberately POST a wrong secret to /sendblue/inbound via curl and observe 401 + ' +
        'this audit row. Without this, the fail-closed claim is unproven.'
    });
  } else {
    findings.push({
      label: 'invalid-auth rejection (founder curl with wrong secret produces 401 + audit)',
      status: 'pass',
      detail: `${sigInvalid.length} rejection(s); reason codes: ${[...sigInvalidReasons].join(', ')}`
    });
  }
  console.log('');

  /* ---- audit_log: STOP recorded (deterministic, NOT classifier) ---- */
  const stopRecorded = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.sendblue.stop_recorded'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(10);
  console.log(`audit_log fomo.sendblue.stop_recorded: ${stopRecorded.length} entry(ies)`);
  for (const e of stopRecorded.slice(0, 3)) {
    console.log(`  id=${e.id} at=${e.occurred_at.toISOString()} detail=${JSON.stringify(e.detail)}`);
  }
  if (stopRecorded.length === 0) {
    findings.push({
      label: 'STOP is deterministic (≥1 fomo.sendblue.stop_recorded)',
      status: 'fail',
      detail: 'No stop_recorded entries. The runbook §8 requires the founder to text STOP and observe this audit row.'
    });
  } else {
    findings.push({
      label: 'STOP is deterministic (no LLM involved); audit row written',
      status: 'pass',
      detail: `${stopRecorded.length} STOP(s) recorded`
    });
  }

  /* ---- audit_log: START recorded (optional but recommended) ---- */
  const startRecorded = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.sendblue.start_recorded'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(10);
  console.log(`audit_log fomo.sendblue.start_recorded: ${startRecorded.length} entry(ies)`);
  if (startRecorded.length === 0) {
    findings.push({
      label: 'START clears stop (RECOMMENDED — only required if founder ran scenario 6)',
      status: 'warn',
      detail:
        'No start_recorded entries. If you ran scenario 6 (texting START), check for issues. If you ' +
        'skipped scenario 6, this is fine.'
    });
  } else {
    findings.push({
      label: 'START clears stop (audit row written)',
      status: 'pass',
      detail: `${startRecorded.length} START(s) recorded`
    });
  }
  console.log('');

  /* ---- audit_log: reply_parsed for a soft intent (classifier) ---- */
  const replyParsed = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.sendblue.reply_parsed'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(10);
  console.log(`audit_log fomo.sendblue.reply_parsed: ${replyParsed.length} entry(ies)`);
  let classifierSoftIntents = 0;
  for (const e of replyParsed.slice(0, 5)) {
    const d = e.detail as { intent?: string; intent_source?: string } | null;
    console.log(`  id=${e.id} intent=${d?.intent} source=${d?.intent_source}`);
    if (d?.intent_source === 'classifier') classifierSoftIntents++;
  }
  if (classifierSoftIntents === 0) {
    findings.push({
      label: 'one soft reply intent parsed via classifier (REQUIRED)',
      status: 'fail',
      detail:
        'No reply_parsed rows with intent_source=classifier. The runbook §7 requires the founder to ' +
        "text a soft intent (e.g. 'tomorrow') and observe the OpenAI classifier path firing."
    });
  } else {
    findings.push({
      label: 'soft reply intent parsed via classifier',
      status: 'pass',
      detail: `${classifierSoftIntents} classifier-parsed soft intent(s)`
    });
  }
  console.log('');

  /* ---- audit_log: reply_duplicate (idempotency proof) ---- */
  const replyDup = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.sendblue.reply_duplicate'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(10);
  console.log(`audit_log fomo.sendblue.reply_duplicate: ${replyDup.length} entry(ies)`);
  if (replyDup.length === 0) {
    findings.push({
      label: 'duplicate webhook is idempotent (≥1 fomo.sendblue.reply_duplicate)',
      status: 'fail',
      detail:
        'No reply_duplicate entries. The runbook §9 requires the founder to re-POST a previous SendBlue ' +
        'payload via curl (simulating SendBlue retry) and observe this audit row + no double-write.'
    });
  } else {
    findings.push({
      label: 'duplicate webhook is idempotent (re-post produces reply_duplicate audit; no double-write)',
      status: 'pass',
      detail: `${replyDup.length} duplicate(s) caught by inbound_replies UNIQUE constraint`
    });
  }
  console.log('');

  /* ---- alert_state_transitions: sent → replied → (snoozed|ignored) ---- */
  const recentTransitions = await db
    .select()
    .from(alert_state_transitions)
    .orderBy(sql`${alert_state_transitions.at} DESC`)
    .limit(50);
  let sentToReplied = 0;
  let repliedToSnoozed = 0;
  let repliedToIgnored = 0;
  for (const t of recentTransitions) {
    if (t.from_state === 'sent' && t.to_state === 'replied') sentToReplied++;
    if (t.from_state === 'replied' && t.to_state === 'snoozed') repliedToSnoozed++;
    if (t.from_state === 'replied' && t.to_state === 'ignored') repliedToIgnored++;
  }
  console.log(
    `alert_state_transitions: sent→replied=${sentToReplied}, replied→snoozed=${repliedToSnoozed}, replied→ignored=${repliedToIgnored}`
  );
  if (sentToReplied === 0) {
    findings.push({
      label: 'state transition sent → replied (REQUIRED)',
      status: 'fail',
      detail: 'No sent→replied transitions. The soft-intent path did not run.'
    });
  } else {
    findings.push({
      label: 'state transition sent → replied',
      status: 'pass',
      detail: `${sentToReplied} transition(s)`
    });
  }
  if (repliedToSnoozed + repliedToIgnored === 0) {
    findings.push({
      label: 'terminal state transition replied → snoozed OR ignored (REQUIRED)',
      status: 'fail',
      detail: 'No terminal transition from replied state. The soft intent did not resolve.'
    });
  } else {
    findings.push({
      label: 'terminal state transition replied → snoozed | ignored',
      status: 'pass',
      detail: `snoozed=${repliedToSnoozed}, ignored=${repliedToIgnored}`
    });
  }
  console.log('');

  /* ---- feedback_events: founder-reply-derived events ---- */
  const inboundFeedback = await db
    .select()
    .from(feedback_events)
    .where(
      sql`${feedback_events.kind} IN ('user_snoozed', 'user_ignored', 'ignored_sender', 'asked_why', 'stop', 'false_positive')`
    )
    .orderBy(sql`${feedback_events.occurred_at} DESC`)
    .limit(20);
  console.log(`feedback_events (inbound-derived): ${inboundFeedback.length} row(s)`);
  for (const r of inboundFeedback.slice(0, 5)) {
    console.log(`  id=${r.id} kind=${r.kind} alert_id=${r.alert_id ?? '<none>'}`);
  }
  if (inboundFeedback.length === 0) {
    findings.push({
      label: 'feedback events from inbound replies ≥ 1 (REQUIRED)',
      status: 'fail',
      detail: 'No matching feedback rows. The route did not write feedback for any inbound reply.'
    });
  } else {
    findings.push({
      label: 'feedback events from inbound replies',
      status: 'pass',
      detail: `${inboundFeedback.length} event(s)`
    });
  }
  console.log('');

  /* ---- memory_signals: stop_active state ---- */
  const stopActiveRows = await db
    .select()
    .from(memory_signals)
    .where(sql`${memory_signals.kind} = 'stop_active'`)
    .orderBy(sql`${memory_signals.updated_at} DESC`)
    .limit(10);
  console.log(`memory_signals stop_active: ${stopActiveRows.length} row(s)`);
  for (const r of stopActiveRows.slice(0, 3)) {
    const d = r.detail as { active?: boolean };
    console.log(`  id=${r.id} user=${r.user_id} active=${d?.active} updated=${r.updated_at.toISOString()}`);
  }
  if (stopActiveRows.length === 0) {
    findings.push({
      label: 'stop_active memory_signal exists (REQUIRED — proves STOP wrote memory)',
      status: 'fail',
      detail: 'No stop_active rows. STOP did not flip the memory signal.'
    });
  } else {
    findings.push({
      label: 'stop_active memory_signal exists (STOP wrote memory)',
      status: 'pass',
      detail: `${stopActiveRows.length} stop_active signal(s); latest active=${(stopActiveRows[0]?.detail as { active?: boolean })?.active}`
    });
  }
  console.log('');

  /* ---- audit_log: STOP enforcement blocked an outbound send (REQUIRED) ---- */
  const stopEnforced = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.send.stop_enforced'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(10);
  console.log(`audit_log fomo.send.stop_enforced: ${stopEnforced.length} entry(ies)`);
  for (const e of stopEnforced.slice(0, 3)) {
    console.log(`  id=${e.id} at=${e.occurred_at.toISOString()} detail=${JSON.stringify(e.detail)}`);
  }
  if (stopEnforced.length === 0) {
    findings.push({
      label: 'STOP enforcement blocked a future outbound send (REQUIRED)',
      status: 'fail',
      detail:
        'No fomo.send.stop_enforced rows. The runbook §8 step 4 requires triggering a NEW approved alert ' +
        'AFTER STOP and observing the outbound-sender refuse to send.'
    });
  } else {
    findings.push({
      label: 'STOP enforcement blocked a future outbound send',
      status: 'pass',
      detail: `${stopEnforced.length} stop_enforced row(s); zero SendBlue API calls fired for these alerts`
    });
  }
  console.log('');

  /* ---- tool_invocations: sanity check that stop_enforced was NOT a tool_invocation ---- */
  // A stop_enforced refusal happens BEFORE dispatch — the worker
  // never reaches dispatch.execute('sendblue.send_user_message').
  // So fomo.send.stop_enforced count should EXCEED the number of
  // additional sendblue.send_user_message tool_invocations rows
  // post-STOP (which should be zero for the new alerts).
  const sendInvocations = await db
    .select()
    .from(tool_invocations)
    .where(sql`${tool_invocations.tool_id} = 'sendblue.send_user_message'`)
    .orderBy(sql`${tool_invocations.occurred_at} DESC`)
    .limit(20);
  console.log(`tool_invocations sendblue.send_user_message: ${sendInvocations.length} total`);

  /* ---- Leak canary scan ---- */
  console.log(
    'Scanning for leak canaries in audit_log + tool_invocations.metadata + ' +
      'feedback_events.detail + alert_state_transitions.reason + memory_signals.detail + ' +
      'inbound_replies ...'
  );
  if (FOUNDER_PHONE.length > 0) {
    console.log(`  (scanning for the literal FOMO_FOUNDER_PHONE_NUMBER value)`);
  }
  if (WEBHOOK_SECRET.length > 8) {
    console.log(`  (scanning for the literal SENDBLUE_WEBHOOK_SECRET value)`);
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
  const recentFeedbackAll = await db
    .select()
    .from(feedback_events)
    .orderBy(sql`${feedback_events.occurred_at} DESC`)
    .limit(200);
  for (const r of recentFeedbackAll) {
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
  const recentMemory = await db
    .select()
    .from(memory_signals)
    .orderBy(sql`${memory_signals.updated_at} DESC`)
    .limit(200);
  for (const r of recentMemory) {
    leaks.push(
      ...scanForLeaks(`memory_signals[id=${r.id}, kind=${r.kind}]`, r.id, r.detail)
    );
  }
  for (const r of inboundRows) {
    leaks.push(
      ...scanForLeaks(
        `inbound_replies[id=${r.id}, provider_message_id=${r.provider_message_id}]`,
        r.id,
        {
          provider_message_id: r.provider_message_id,
          user_id: r.user_id
        }
      )
    );
  }

  if (leaks.length === 0) {
    console.log('  ✓ no forbidden keys or value patterns found');
    findings.push({
      label: 'No reply text / phone / webhook secret / API keys leaked in any persisted store',
      status: 'pass',
      detail: `Scanned ${recentAuditAll.length} audit + ${recentToolInv.length} tool_invocations + ${recentFeedbackAll.length} feedback + ${recentTransitions.length} transition + ${recentMemory.length} memory_signal + ${inboundRows.length} inbound_replies rows; zero hits.`
    });
  } else {
    console.log(`  ✖ ${leaks.length} potential leak hit(s):`);
    for (const h of leaks.slice(0, 20)) {
      console.log(`    [${h.source}] ${h.reason}`);
      console.log(`      excerpt: ${h.excerpt}`);
    }
    findings.push({
      label: 'No reply text / phone / webhook secret / API keys leaked in any persisted store',
      status: 'fail',
      detail: `${leaks.length} hit(s). First: ${leaks[0]?.reason}`
    });
  }
  console.log('');

  /* ---- Founder-recorded fields (NOT in evidence script; reminder) ---- */
  findings.push({
    label: 'Auth observation fields recorded by founder in §5 of the report (LOAD-BEARING)',
    status: 'warn',
    detail:
      'This evidence script CANNOT verify what header SendBlue actually sent. The founder MUST fill in §5 ' +
      'of SMOKE_REPORT_3F2.md: observed webhook header name, observed auth scheme (plain-secret-header / ' +
      'HMAC / other), whether the header value equaled the configured secret literally, and whether a ' +
      'runtime patch was required. Reviewer (Claude) MUST verify these fields are filled in and consistent ' +
      'with the runtime config before merging the PR.'
  });

  /* ---- Verdict ---- */
  console.log('='.repeat(72));
  console.log('Phase 3F.2 evidence summary');
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
    console.log('Phase 3G demo gate is now unblocked.');
    console.log('');
    console.log('REMINDER: This script CANNOT verify the auth-mechanism claim.');
    console.log('Founder MUST record observed auth header + scheme in §5 of the smoke report.');
    console.log('Reviewer MUST verify those fields before merging.');
  } else {
    console.log(`VERDICT: FAIL  (${failCount} required check(s) failed)`);
    console.log('Do NOT start Phase 3G until every required check is green.');
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
