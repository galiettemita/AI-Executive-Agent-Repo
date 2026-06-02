// Phase v0.5.3 item #4 — SendBlue webhook-delivery reconciliation
// (on-demand ops script).
//
// v0.5.2 smoke incident 2026-06-01: Morris's STOP iMessage reached
// SendBlue (their /api/v2/messages confirms it) but our /sendblue/inbound
// webhook never fired — because our server was down at the time AND
// SendBlue's retries exhausted. We had no audit row indicating a gap
// existed; the gap was only discovered by manually querying SendBlue's
// API 11h later.
//
// This script closes that visibility gap:
//   1. Fetches recent inbound messages from SendBlue (/api/v2/messages
//      with is_outbound=false, paginated)
//   2. Diffs against our audit_log (join on message_handle =
//      provider_message_id within fomo.sendblue.inbound_received)
//   3. For each gap (SendBlue has it; we don't), audits
//      fomo.sendblue.delivery_gap_detected with safe detail
//   4. Prints a summary
//
// Per founder correction #4: this is ON-DEMAND ONLY in v0.5.3. A
// periodic background worker is a future phase. Operator runs:
//   pnpm --filter @brevio/fomo run ops:reconcile-sendblue
//
// Optional --window-hours <N> (default 24).

import { sql } from 'drizzle-orm';

import { loadDbClient, type DbClientResult } from '../src/db/client.js';
import { PostgresAuditStore } from '../src/db/stores/audit-postgres.js';
import { audit_log } from '../src/db/schema.js';
import {
  reconcileSendBlue,
  phoneSlugFromMessage,
  type SendBlueMessage
} from '../src/adapters/sendblue/reconcile.js';

interface ParsedArgs {
  window_hours: number;
}

function parseArgs(argv: readonly string[]): ParsedArgs {
  const out: { window_hours?: number } = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const next = argv[i + 1];
    if (a === '--window-hours' && typeof next === 'string') {
      const n = Number(next);
      if (!Number.isFinite(n) || n <= 0 || n > 168) {
        throw new Error('--window-hours must be a positive number ≤ 168 (one week ceiling)');
      }
      out.window_hours = n;
      i++;
    }
  }
  return { window_hours: out.window_hours ?? 24 };
}

async function mainCli(): Promise<void> {
  const args = parseArgs(process.argv.slice(2));

  if (!(process.env.DATABASE_URL ?? '').trim()) {
    process.stderr.write('[ops-reconcile-sendblue] DATABASE_URL is not set. Source .env first.\n');
    process.exit(2);
  }
  const apiKeyId = process.env.SENDBLUE_API_KEY_ID;
  const apiSecretKey = process.env.SENDBLUE_API_SECRET_KEY;
  if (!apiKeyId || !apiSecretKey) {
    process.stderr.write(
      '[ops-reconcile-sendblue] SENDBLUE_API_KEY_ID + SENDBLUE_API_SECRET_KEY required.\n'
    );
    process.exit(2);
  }

  const dbResult: DbClientResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    process.stderr.write(`[ops-reconcile-sendblue] DB unavailable: ${dbResult.reason}\n`);
    process.exit(1);
  }

  const auditStore = new PostgresAuditStore(dbResult.client);
  const db = dbResult.client;

  try {
    const result = await reconcileSendBlue({
      apiKeyId,
      apiSecretKey,
      windowHours: args.window_hours,
      fetchAuditedHandles: async (sinceMs: number) => {
        // Pull every message_handle we've audited in the window via
        // fomo.sendblue.inbound_received. We pull the detail jsonb
        // and extract; the inbound audit row's `detail` doesn't
        // currently store message_handle directly (it stores body_bytes
        // + secret_header_*), but reply_parsed / stop_recorded /
        // start_recorded DO contain provider_message_id. So we look
        // across all sendblue.* actions that we know carry the handle.
        const since = new Date(sinceMs);
        const rows = await db
          .select({ detail: audit_log.detail, action: audit_log.action })
          .from(audit_log)
          .where(
            sql`${audit_log.occurred_at} >= ${since.toISOString()}::timestamptz
                AND ${audit_log.action} IN (
                  'fomo.sendblue.stop_recorded',
                  'fomo.sendblue.start_recorded',
                  'fomo.sendblue.reply_parsed',
                  'fomo.sendblue.reply_duplicate',
                  'fomo.sendblue.reply_unclear',
                  'fomo.sendblue.reply_unauthorized'
                )`
          );
        const handles = new Set<string>();
        for (const r of rows) {
          const d = (r.detail ?? {}) as { provider_message_id?: unknown };
          if (typeof d.provider_message_id === 'string') {
            handles.add(d.provider_message_id);
          }
        }
        return handles;
      },
      recordGap: async (msg: SendBlueMessage) => {
        await auditStore.write({
          actor_user_id: null,
          actor_ip: null,
          actor_user_agent: null,
          action: 'fomo.sendblue.delivery_gap_detected',
          target: `sendblue:${msg.message_handle}`,
          result: 'failure',
          detail: {
            provider_message_id: msg.message_handle,
            from_slug: phoneSlugFromMessage(msg),
            date_sent_iso: msg.date_sent,
            service: msg.service,
            sendblue_status: msg.status
            // NEVER msg.content (the message body); NEVER msg.from_number (full E.164)
          }
        });
      }
    });

    process.stdout.write('\n');
    process.stdout.write('✓ SendBlue reconciliation complete.\n');
    process.stdout.write(`  window_hours:        ${args.window_hours}\n`);
    process.stdout.write(`  sendblue_inbounds:   ${result.sendblue_inbound_count}\n`);
    process.stdout.write(`  audit_handles_seen:  ${result.audit_handles_in_window}\n`);
    process.stdout.write(`  gaps_found:          ${result.gaps_found}\n`);
    if (result.gap_handles.length > 0) {
      process.stdout.write('\n  Gap handles (each audited as fomo.sendblue.delivery_gap_detected):\n');
      for (const h of result.gap_handles) {
        process.stdout.write(`    - ${h}\n`);
      }
      process.stdout.write(
        '\n  Each gap is a SendBlue-confirmed inbound message that our /sendblue/inbound\n' +
          '  webhook never recorded. Most common cause: server was down at delivery time +\n' +
          "  SendBlue's retries exhausted. Investigate the server uptime in the window\n" +
          '  AND consider whether the message intent needs a synthetic replay (see v0.5.2\n' +
          '  SMOKE_REPORT §6 caveat for the established Path A pattern).\n'
      );
    } else {
      process.stdout.write('\n  No gaps — every SendBlue inbound in the window has a matching audit row.\n');
    }
    process.stdout.write('\n');
  } finally {
    await dbResult.pool.end();
  }
}

// Run only when invoked as a script (not when imported by tests).
if (
  process.argv[1] &&
  // import.meta.url is the file URL; argv[1] is the OS path of the
  // entry script. We compare path tails to avoid platform differences.
  process.argv[1].endsWith('ops-reconcile-sendblue.ts')
) {
  mainCli().catch((err) => {
    process.stderr.write(
      `[ops-reconcile-sendblue] fatal: ${err instanceof Error ? err.message : String(err)}\n`
    );
    process.exit(1);
  });
}
