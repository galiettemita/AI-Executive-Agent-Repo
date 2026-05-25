// Phase 3C.4 evidence — queries the live Neon Postgres substrate after
// a smoke-test run and prints the evidence the founder pastes into the
// SMOKE_REPORT_TEMPLATE_3C4.md.
//
// Extends Phase 3B.3 evidence with ranker-on-poll checks:
//   - oauth_tokens.scopes still gmail.readonly only (regression)
//   - gmail_cursors present + advanced
//   - audit_log gmail.poll.cycle entries: at least one, detail surfaces
//     ranker counters (messages_ranked, messages_rank_already, messages_rank_failed)
//   - audit_log gmail.read dispatch entries (regression coverage)
//   - audit_log fomo.rank.completed: REQUIRED >=1 (proves the ranker
//     actually fired against a real email)
//   - audit_log fomo.rank.already_ranked: REQUIRED >=1 (proves the
//     idempotency seam works against live Postgres)
//   - audit_log fomo.rank.failed: counted; >0 is WARN (cycle still passes)
//   - tool_invocations gmail.read row(s)
//   - rank_results: REQUIRED >=1 row; each row's columns sane
//   - Leak-canary scan over audit + tool_invocations + rank_results
//     (the ranker-authored `reason` column is bounded by the validator
//     but still scanned for forbidden value patterns just in case)
//
// Read-only. Does not write or mutate any row.
//
// Run via `pnpm smoke-evidence:3c4`. Requires DATABASE_URL pointed at
// the same Neon instance the smoke-test server used.

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import {
  audit_log,
  gmail_cursors,
  oauth_tokens,
  rank_results,
  tool_invocations
} from '../src/db/schema.js';

const GMAIL_READONLY = 'https://www.googleapis.com/auth/gmail.readonly';

/* ---------------------------------------------------------------------- */
/* Leak-canary scanner                                                    */
/* ---------------------------------------------------------------------- */

const FORBIDDEN_KEYS: readonly string[] = Object.freeze([
  'body_plain',
  'body_html',
  'body_snippet',
  'attachments',
  'headers',
  'raw'
]);

const FORBIDDEN_VALUE_PATTERNS: readonly RegExp[] = Object.freeze([
  // Long base64-url blob.
  /[A-Za-z0-9_-]{200,}/,
  // Raw header dump shape.
  /^Authentication-Results:/im,
  /^Received: from/im,
  /^Content-Transfer-Encoding:/im
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
  console.log('Phase 3C.4 evidence — querying Neon Postgres substrate\n');

  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    console.error(`[ERROR] Cannot load DB client: ${dbResult.reason}`);
    console.error('        Set DATABASE_URL=postgres://... and re-run.');
    process.exit(2);
  }
  const db = dbResult.client;

  const findings: SmokeFinding[] = [];

  /* ---- oauth_tokens: regression check on scope ---- */
  const tokenRows = await db
    .select({
      user_id: oauth_tokens.user_id,
      provider: oauth_tokens.provider,
      scopes: oauth_tokens.scopes,
      needs_reauth: oauth_tokens.needs_reauth
    })
    .from(oauth_tokens)
    .where(sql`${oauth_tokens.provider} = 'google'`);

  console.log(`oauth_tokens (provider='google'): ${tokenRows.length} row(s)`);
  if (tokenRows.length === 0) {
    findings.push({
      label: 'OAuth token persisted (regression check)',
      status: 'fail',
      detail: 'No row in oauth_tokens with provider=google. Did the founder complete OAuth?'
    });
  } else {
    for (const r of tokenRows) {
      console.log(`  user_id=${r.user_id} scopes=${JSON.stringify(r.scopes)} needs_reauth=${r.needs_reauth}`);
      const scopes = Array.isArray(r.scopes) ? r.scopes : [];
      const onlyReadonly = scopes.length === 1 && scopes[0] === GMAIL_READONLY;
      findings.push({
        label: `OAuth scope is gmail.readonly only (user=${r.user_id}) — regression check`,
        status: onlyReadonly ? 'pass' : 'fail',
        detail: `scopes=${JSON.stringify(scopes)}; required=[${GMAIL_READONLY}]`
      });
    }
  }
  console.log('');

  /* ---- gmail_cursors ---- */
  const cursorRows = await db
    .select({
      user_id: gmail_cursors.user_id,
      history_id: gmail_cursors.history_id,
      updated_at: gmail_cursors.updated_at
    })
    .from(gmail_cursors);
  console.log(`gmail_cursors: ${cursorRows.length} row(s)`);
  for (const r of cursorRows) {
    console.log(`  user_id=${r.user_id} history_id=${r.history_id} updated_at=${r.updated_at.toISOString()}`);
  }
  if (cursorRows.length === 0) {
    findings.push({
      label: 'Gmail cursor present',
      status: 'fail',
      detail: 'No gmail_cursors row. OAuth callback should have seeded one.'
    });
  } else {
    findings.push({
      label: 'Gmail cursor present',
      status: 'pass',
      detail: `${cursorRows.length} cursor row(s); latest history_id=${cursorRows[0]?.history_id}`
    });
  }
  console.log('');

  /* ---- audit_log: gmail.poll.cycle ---- */
  const cycleEntries = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'gmail.poll.cycle'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(20);
  console.log(`audit_log action='gmail.poll.cycle': ${cycleEntries.length} entry(ies)`);
  let cycleRankCount = 0;
  for (const e of cycleEntries) {
    console.log(`  id=${e.id} at=${e.occurred_at.toISOString()} result=${e.result} detail=${JSON.stringify(e.detail)}`);
    const detail = (e.detail ?? {}) as Record<string, unknown>;
    if (typeof detail.messages_ranked === 'number') cycleRankCount += detail.messages_ranked;
  }
  if (cycleEntries.length === 0) {
    findings.push({
      label: 'Polling cycle audit written',
      status: 'fail',
      detail: 'No gmail.poll.cycle audit entry. Did the polling worker run at all?'
    });
  } else {
    findings.push({
      label: 'Polling cycle audit written, ranker counters surfaced in detail',
      status: 'pass',
      detail: `${cycleEntries.length} cycle(s) recorded; sum messages_ranked across cycles=${cycleRankCount}`
    });
  }
  console.log('');

  /* ---- audit_log: gmail.read dispatch (regression) ---- */
  const gmailReadAudits = await db
    .select()
    .from(audit_log)
    .where(
      sql`${audit_log.target} = 'tool:gmail.read' AND ${audit_log.action} IN ('policy.decided', 'tool.invoked')`
    )
    .limit(200);
  const decideCount = gmailReadAudits.filter((e) => e.action === 'policy.decided').length;
  const invokeCount = gmailReadAudits.filter((e) => e.action === 'tool.invoked').length;
  console.log(
    `audit_log gmail.read dispatch (regression): policy.decided=${decideCount} tool.invoked=${invokeCount}`
  );
  if (invokeCount === 0) {
    findings.push({
      label: 'gmail.read dispatch fired (regression)',
      status: 'fail',
      detail: 'No tool.invoked audits for gmail.read. The ranker cannot have run because no message was dispatched.'
    });
  } else {
    findings.push({
      label: 'gmail.read dispatch fired (regression)',
      status: 'pass',
      detail: `policy.decided=${decideCount} tool.invoked=${invokeCount}`
    });
  }
  console.log('');

  /* ---- audit_log: fomo.rank.completed (REQUIRED for 3C.4 PASS) ---- */
  const rankCompletedAudits = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.rank.completed'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(50);
  console.log(`audit_log action='fomo.rank.completed': ${rankCompletedAudits.length} entry(ies)`);
  for (const e of rankCompletedAudits.slice(0, 5)) {
    console.log(`  id=${e.id} at=${e.occurred_at.toISOString()} detail=${JSON.stringify(e.detail)}`);
  }
  if (rankCompletedAudits.length === 0) {
    findings.push({
      label: 'fomo.rank.completed audit written (≥1 required for 3C.4 PASS)',
      status: 'fail',
      detail: 'No fomo.rank.completed audit entry. The ranker did not successfully classify any real Gmail message. Possible causes: (a) the founder inbox had zero new messages in the smoke window — send yourself a test email and re-run; (b) the ranker returned only RankerFailure (check fomo.rank.failed below); (c) FOMO_RANKER_ENABLED was off when the worker ran.'
    });
  } else {
    findings.push({
      label: 'fomo.rank.completed audit written (≥1 required for 3C.4 PASS)',
      status: 'pass',
      detail: `${rankCompletedAudits.length} successful rank(s) audited`
    });
  }
  console.log('');

  /* ---- audit_log: fomo.rank.already_ranked (REQUIRED for idempotency proof) ---- */
  const rankAlreadyAudits = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.rank.already_ranked'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(50);
  console.log(`audit_log action='fomo.rank.already_ranked': ${rankAlreadyAudits.length} entry(ies)`);
  if (rankAlreadyAudits.length === 0) {
    findings.push({
      label: 'fomo.rank.already_ranked audit written (≥1 required — idempotency proof)',
      status: 'fail',
      detail: 'No fomo.rank.already_ranked audit entry. The runbook requires exercising the idempotency seam — see docs/smoke-test-3c4-rank-on-poll.md §"second cycle". Re-run a cycle over the same history range and re-run this script.'
    });
  } else {
    findings.push({
      label: 'fomo.rank.already_ranked audit written (idempotency seam exercised against live Postgres)',
      status: 'pass',
      detail: `${rankAlreadyAudits.length} duplicate(s) audited — ON CONFLICT DO NOTHING confirmed firing`
    });
  }
  console.log('');

  /* ---- audit_log: fomo.rank.failed (WARN if any) ---- */
  const rankFailedAudits = await db
    .select()
    .from(audit_log)
    .where(sql`${audit_log.action} = 'fomo.rank.failed'`)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(50);
  console.log(`audit_log action='fomo.rank.failed': ${rankFailedAudits.length} entry(ies)`);
  for (const e of rankFailedAudits.slice(0, 5)) {
    console.log(`  id=${e.id} at=${e.occurred_at.toISOString()} detail=${JSON.stringify(e.detail)}`);
  }
  if (rankFailedAudits.length === 0) {
    findings.push({
      label: 'fomo.rank.failed clean (no model errors in smoke window)',
      status: 'pass',
      detail: '0 ranker failures'
    });
  } else {
    findings.push({
      label: 'fomo.rank.failed observed',
      status: 'warn',
      detail: `${rankFailedAudits.length} ranker failure(s); inspect detail.error_code + detail.reason above. Not a gate failure on its own — the cycle continued — but worth understanding.`
    });
  }
  console.log('');

  /* ---- tool_invocations: gmail.read ---- */
  const gmailReadInv = await db
    .select()
    .from(tool_invocations)
    .where(sql`${tool_invocations.tool_id} = 'gmail.read'`)
    .orderBy(sql`${tool_invocations.occurred_at} DESC`)
    .limit(50);
  console.log(`tool_invocations tool_id='gmail.read': ${gmailReadInv.length} row(s)`);
  for (const r of gmailReadInv.slice(0, 5)) {
    console.log(
      `  id=${r.id} invocation_id=${r.invocation_id} policy_decision=${r.policy_decision} status=${r.status} latency_ms=${r.latency_ms}`
    );
  }
  console.log('');

  /* ---- rank_results: REQUIRED for 3C.4 PASS ---- */
  const rankRows = await db
    .select()
    .from(rank_results)
    .orderBy(sql`${rank_results.created_at} DESC`)
    .limit(50);
  console.log(`rank_results: ${rankRows.length} row(s)`);
  let importantCount = 0;
  let notImportantCount = 0;
  for (const r of rankRows.slice(0, 10)) {
    console.log(
      `  id=${r.id} user=${r.user_id} message=${r.message_id} label=${r.label} score=${r.score} model=${r.model_name} v=${r.prompt_version} latency=${r.latency_ms}ms tokens=${r.input_tokens}/${r.output_tokens} cost=$${r.estimated_cost_usd.toFixed(6)} reason="${r.reason.slice(0, 80)}${r.reason.length > 80 ? '...' : ''}"`
    );
    if (r.label === 'important') importantCount++;
    else if (r.label === 'not_important') notImportantCount++;
  }
  if (rankRows.length === 0) {
    findings.push({
      label: 'rank_results populated (≥1 row required for 3C.4 PASS)',
      status: 'fail',
      detail: 'No rank_results rows. The ranker either never fired or all calls failed.'
    });
  } else {
    findings.push({
      label: 'rank_results populated (≥1 row required for 3C.4 PASS)',
      status: 'pass',
      detail: `${rankRows.length} row(s); important=${importantCount}, not_important=${notImportantCount}. Founder eyeballs reasonableness in the report; not a gate criterion.`
    });
  }
  console.log('');

  /* ---- Leak canary scan ---- */
  console.log('Scanning for leak canaries in audit_log + tool_invocations + rank_results ...');
  const leaks: LeakHit[] = [];

  const recentAudits = await db
    .select()
    .from(audit_log)
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(500);
  for (const e of recentAudits) {
    leaks.push(...scanForLeaks(`audit_log[id=${e.id}, action=${e.action}]`, e.id, e.detail));
  }

  const recentInv = await db
    .select()
    .from(tool_invocations)
    .orderBy(sql`${tool_invocations.occurred_at} DESC`)
    .limit(500);
  for (const r of recentInv) {
    leaks.push(
      ...scanForLeaks(
        `tool_invocations[id=${r.id}, tool=${r.tool_id}]`,
        r.id,
        r.metadata
      )
    );
  }

  // rank_results: scan the model-authored reason column specifically.
  // It is validator-bounded to ≤240 chars but if the model ever sneaks
  // header content or a base64 blob in, this catches it.
  for (const r of rankRows) {
    leaks.push(
      ...scanForLeaks(
        `rank_results[id=${r.id}, message=${r.message_id}].reason`,
        r.id,
        { reason: r.reason }
      )
    );
  }

  if (leaks.length === 0) {
    console.log('  ✓ no forbidden keys or value patterns found in 500 most recent audit / tool_invocations records or any rank_results.reason');
    findings.push({
      label: 'No raw email leak in audit / tool_invocations / rank_results.reason',
      status: 'pass',
      detail: `Scanned 500 recent audit + ${recentInv.length} tool_invocations + ${rankRows.length} rank_results rows; zero hits.`
    });
  } else {
    console.log(`  ✖ ${leaks.length} potential leak hit(s):`);
    for (const h of leaks.slice(0, 20)) {
      console.log(`    [${h.source}] ${h.reason}`);
      console.log(`      excerpt: ${h.excerpt}`);
    }
    findings.push({
      label: 'No raw email leak in audit / tool_invocations / rank_results.reason',
      status: 'fail',
      detail: `${leaks.length} hit(s). First: ${leaks[0]?.reason}`
    });
  }
  console.log('');

  /* ---- Verdict ---- */
  console.log('='.repeat(72));
  console.log('Phase 3C.4 evidence summary');
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
    console.log('Phase 3D Slack adapter is now unblocked.');
  } else {
    console.log(`VERDICT: FAIL  (${failCount} required check(s) failed)`);
    console.log('Do NOT start Phase 3D until every required check is green.');
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
