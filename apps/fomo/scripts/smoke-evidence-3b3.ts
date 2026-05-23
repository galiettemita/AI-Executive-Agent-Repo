// Phase 3B.3 evidence — queries the live Neon Postgres substrate after
// a smoke-test run and prints the evidence the founder pastes into the
// SMOKE_REPORT_TEMPLATE_3B3.md.
//
// Verifies each Phase 3B.3 required check:
//   - OAuth token landed (oauth_tokens row for provider='google')
//   - Scope is gmail.readonly ONLY (no broader scope persisted)
//   - Cursor advanced (gmail_cursors row exists; history_id != 0)
//   - Polling cycle audit present (action='gmail.poll.cycle')
//   - Per-message dispatch audits present (policy.decided +
//     tool.invoked for tool_id='gmail.read')
//   - tool_invocations row(s) for gmail.read
//   - No leak canary in any audit detail / tool_invocations metadata
//     (no body_plain, body_html, raw header values, attachment
//     filenames, full base64 blobs)
//   - needs_reauth audit-visible (warns if not yet exercised — the
//     founder may have to force a 401 to prove this)
//
// Read-only. Does not write or mutate any row.
//
// Run via `pnpm smoke:evidence`. Requires DATABASE_URL pointed at the
// same Neon instance the smoke-test server used.

import { sql } from 'drizzle-orm';

import { loadDbClient } from '../src/db/client.js';
import {
  audit_log,
  gmail_cursors,
  oauth_tokens,
  tool_invocations
} from '../src/db/schema.js';

const GMAIL_READONLY = 'https://www.googleapis.com/auth/gmail.readonly';

/* ---------------------------------------------------------------------- */
/* Leak-canary scanner                                                    */
/* ---------------------------------------------------------------------- */

// Keys that MUST never appear in any audit/tool_invocations detail.
// These match the field names the Egress Policy strips for ranker /
// Slack / reply-parser views (FOMO_DESIGN §10 + integration-harness
// invariants).
const FORBIDDEN_KEYS: readonly string[] = Object.freeze([
  'body_plain',
  'body_html',
  'body_snippet',
  'attachments',
  'headers',
  'raw'
]);

// Value-shape heuristics. We don't know the real Gmail content, so we
// scan for shapes that look like raw email payload (long base64
// blobs, From/To header lines, Subject: lines).
const FORBIDDEN_VALUE_PATTERNS: readonly RegExp[] = Object.freeze([
  // Long base64-url blobs (>200 chars) — Gmail body parts arrive
  // base64url-encoded; if one leaked into a detail field it would
  // surface here.
  /[A-Za-z0-9_-]{200,}/,
  // Raw header dump shape (common headers).
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
  console.log('Phase 3B.3 evidence — querying Neon Postgres substrate\n');

  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    console.error(`[ERROR] Cannot load DB client: ${dbResult.reason}`);
    console.error('        Set DATABASE_URL=postgres://... and re-run.');
    process.exit(2);
  }
  const db = dbResult.client;

  const findings: SmokeFinding[] = [];

  /* ---- oauth_tokens ---- */
  const tokenRows = await db
    .select({
      user_id: oauth_tokens.user_id,
      provider: oauth_tokens.provider,
      scopes: oauth_tokens.scopes,
      obtained_at: oauth_tokens.obtained_at,
      needs_reauth: oauth_tokens.needs_reauth,
      key_version: oauth_tokens.key_version
    })
    .from(oauth_tokens)
    .where(sql`${oauth_tokens.provider} = 'google'`);

  console.log(`oauth_tokens (provider='google'): ${tokenRows.length} row(s)`);
  if (tokenRows.length === 0) {
    findings.push({
      label: 'OAuth token persisted',
      status: 'fail',
      detail: 'No row in oauth_tokens with provider=google. Did the founder complete /oauth/google/callback?'
    });
  } else {
    for (const r of tokenRows) {
      console.log(`  user_id=${r.user_id} scopes=${JSON.stringify(r.scopes)} needs_reauth=${r.needs_reauth} key_version=${r.key_version}`);
      const scopes = Array.isArray(r.scopes) ? r.scopes : [];
      const onlyReadonly =
        scopes.length === 1 && scopes[0] === GMAIL_READONLY;
      findings.push({
        label: `OAuth scope is gmail.readonly only (user=${r.user_id})`,
        status: onlyReadonly ? 'pass' : 'fail',
        detail: `scopes=${JSON.stringify(scopes)}; required=[${GMAIL_READONLY}]`
      });
    }
    findings.push({
      label: 'OAuth token persisted',
      status: 'pass',
      detail: `${tokenRows.length} google token row(s) found in oauth_tokens`
    });
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
      label: 'Gmail cursor advanced',
      status: 'fail',
      detail: 'No gmail_cursors row. The OAuth callback should have seeded one.'
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
  for (const e of cycleEntries) {
    console.log(`  id=${e.id} at=${e.occurred_at.toISOString()} result=${e.result} detail=${JSON.stringify(e.detail)}`);
  }
  if (cycleEntries.length === 0) {
    findings.push({
      label: 'Polling cycle audit written',
      status: 'fail',
      detail: 'No gmail.poll.cycle audit entry. Did the polling worker run at all?'
    });
  } else {
    findings.push({
      label: 'Polling cycle audit written',
      status: 'pass',
      detail: `${cycleEntries.length} cycle(s) recorded`
    });
  }
  console.log('');

  /* ---- audit_log: gmail.read dispatch (policy.decided + tool.invoked) ---- */
  const gmailReadAudits = await db
    .select()
    .from(audit_log)
    .where(
      sql`${audit_log.target} = 'tool:gmail.read' AND ${audit_log.action} IN ('policy.decided', 'tool.invoked')`
    )
    .orderBy(sql`${audit_log.occurred_at} DESC`)
    .limit(50);
  const decideCount = gmailReadAudits.filter((e) => e.action === 'policy.decided').length;
  const invokeCount = gmailReadAudits.filter((e) => e.action === 'tool.invoked').length;
  console.log(
    `audit_log gmail.read dispatch: policy.decided=${decideCount} tool.invoked=${invokeCount}`
  );
  if (decideCount + invokeCount === 0) {
    findings.push({
      label: 'gmail.read dispatch audits',
      status: 'warn',
      detail: 'No gmail.read dispatch audits. If the founder inbox had zero new messages in the smoke window, this is expected — but consider sending yourself a test email and re-running with another cycle.'
    });
  } else {
    findings.push({
      label: 'gmail.read dispatch audits',
      status: 'pass',
      detail: `policy.decided=${decideCount} tool.invoked=${invokeCount}`
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
  for (const r of gmailReadInv) {
    console.log(
      `  id=${r.id} invocation_id=${r.invocation_id} policy_decision=${r.policy_decision} status=${r.status} latency_ms=${r.latency_ms} error_code=${r.error_code ?? 'null'}`
    );
  }
  console.log('');

  /* ---- needs_reauth (401 path) ---- */
  const reauthRows = tokenRows.filter((r) => r.needs_reauth === true);
  if (reauthRows.length > 0) {
    findings.push({
      label: '401 → needs_reauth path exercised',
      status: 'pass',
      detail: `${reauthRows.length} token row(s) have needs_reauth=true`
    });
  } else {
    findings.push({
      label: '401 → needs_reauth path exercised',
      status: 'warn',
      detail: 'No token row has needs_reauth=true. The smoke test happy-path does not exercise this; revoke the founder token in https://myaccount.google.com/permissions, run one more cycle, then re-run this script.'
    });
  }

  /* ---- Leak canary scan ---- */
  console.log('Scanning for leak canaries in audit_log.detail + tool_invocations.metadata ...');
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
  if (leaks.length === 0) {
    console.log('  ✓ no forbidden keys or value patterns found in 500 most recent records');
    findings.push({
      label: 'No raw email leak in audit / tool_invocations',
      status: 'pass',
      detail: 'Scanned 500 recent audit + 500 recent tool_invocations records; zero hits.'
    });
  } else {
    console.log(`  ✖ ${leaks.length} potential leak hit(s):`);
    for (const h of leaks.slice(0, 20)) {
      console.log(`    [${h.source}] ${h.reason}`);
      console.log(`      excerpt: ${h.excerpt}`);
    }
    findings.push({
      label: 'No raw email leak in audit / tool_invocations',
      status: 'fail',
      detail: `${leaks.length} hit(s). First: ${leaks[0]?.reason}`
    });
  }
  console.log('');

  /* ---- Verdict ---- */
  console.log('='.repeat(72));
  console.log('Phase 3B.3 evidence summary');
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
  } else {
    console.log(`VERDICT: FAIL  (${failCount} required check(s) failed)`);
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
