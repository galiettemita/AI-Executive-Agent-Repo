// v0.5.8 smoke diagnostic — print the actual error returned by
// GmailClient.listHistorySince for a given user. The runtime catch
// block at workers/gmail-poll.ts:411 captures the error message into
// the per-user outcomes[] array but never persists or logs it; the
// cycle-level audit only carries the count (users_api_error). This
// script is the bridge that surfaces the silent error.
//
// USAGE:
//   pnpm --filter @brevio/fomo run diagnose:gmail-history -- --user-id founder
//
// Read-only: does NOT mutate gmail_cursors or oauth_tokens.

import { GmailClient, GmailUnauthorizedError, GmailApiError } from '../src/adapters/gmail/client.js';
import { loadCryptoConfig } from '../src/security/token-crypto.js';
import { loadDbClient } from '../src/db/client.js';
import { PostgresTokenStore } from '../src/db/stores/token-postgres.js';

interface ParsedArgs {
  user_id: string;
}

function parseArgs(argv: readonly string[]): ParsedArgs {
  const out: { user_id?: string } = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const next = argv[i + 1];
    if (a === '--user-id' && typeof next === 'string') {
      out.user_id = next;
      i++;
    }
  }
  if (!out.user_id) {
    throw new Error('Missing required --user-id <id> argument');
  }
  return { user_id: out.user_id };
}

async function main(): Promise<void> {
  const args = parseArgs(process.argv.slice(2));

  if (!(process.env.DATABASE_URL ?? '').trim()) {
    process.stderr.write('[diagnose-gmail-history] DATABASE_URL is not set. Source .env first.\n');
    process.exit(2);
  }

  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    process.stderr.write(`[diagnose-gmail-history] DB client load failed: ${dbResult.reason}\n`);
    process.exit(2);
  }

  const cryptoConfig = loadCryptoConfig();
  const tokenStore = new PostgresTokenStore(dbResult.client, cryptoConfig);

  const accessToken = await tokenStore.loadAccessToken(args.user_id, 'google');
  if (accessToken === null) {
    process.stderr.write(
      `[diagnose-gmail-history] No usable access_token for user_id=${args.user_id} (decrypt returned null or row missing).\n`
    );
    process.exit(3);
  }

  const cursorRow = await dbResult.pool.query<{ history_id: string }>(
    'SELECT history_id FROM gmail_cursors WHERE user_id = $1',
    [args.user_id]
  );
  if (cursorRow.rows.length === 0) {
    process.stderr.write(
      `[diagnose-gmail-history] No gmail_cursors row for user_id=${args.user_id}. Cannot call listHistorySince without a startHistoryId.\n`
    );
    process.exit(4);
  }
  const firstRow = cursorRow.rows[0];
  if (!firstRow) {
    process.stderr.write(
      `[diagnose-gmail-history] gmail_cursors query returned zero rows for user_id=${args.user_id}.\n`
    );
    process.exit(4);
  }
  const startHistoryId = firstRow.history_id;
  process.stdout.write(
    `[diagnose-gmail-history] user_id=${args.user_id} startHistoryId=${startHistoryId} access_token_len=${accessToken.length}\n`
  );

  const client = new GmailClient();
  try {
    const result = await client.listHistorySince(accessToken, startHistoryId, { maxResults: 100 });
    process.stdout.write(
      `[diagnose-gmail-history] SUCCESS — latest_history_id=${result.latest_history_id} added_message_count=${result.added_message_ids.length} malformed_labelAdded_skipped=${result.malformed_labelAdded_skipped}\n`
    );
    process.exit(0);
  } catch (err) {
    if (err instanceof GmailUnauthorizedError) {
      process.stderr.write(`[diagnose-gmail-history] GmailUnauthorizedError: ${err.message}\n`);
      process.exit(10);
    }
    if (err instanceof GmailApiError) {
      process.stderr.write(
        `[diagnose-gmail-history] GmailApiError http_status=${err.httpStatus} provider_code=${err.providerCode ?? 'null'} message=${err.message} retryable=${err.retryable}\n`
      );
      process.exit(11);
    }
    process.stderr.write(
      `[diagnose-gmail-history] UNKNOWN error class=${err?.constructor?.name ?? 'unknown'} message=${err instanceof Error ? err.message : String(err)}\n`
    );
    process.exit(12);
  }
}

main().catch((err) => {
  process.stderr.write(`[diagnose-gmail-history] fatal: ${err instanceof Error ? err.message : String(err)}\n`);
  process.exit(1);
});
