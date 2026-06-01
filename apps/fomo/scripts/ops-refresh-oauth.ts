// Ops tool — refresh a user's Google OAuth access_token using their
// stored refresh_token. Unblocks users with `needs_reauth=true` due to
// expired access_token, WITHOUT requiring them to re-OAuth.
//
// v0.5.2 smoke surfaced that the polling worker does not auto-refresh
// expired access_tokens (it checks needs_reauth, skips, but never calls
// refreshAccessToken even though we have the refresh_token stored). The
// full fix (wire refresh into the polling worker) is a v0.5.3 hardening
// candidate. This script is the bridge: founder runs it on-demand to
// re-establish a working access_token for any user.
//
// USAGE:
//   pnpm --filter @brevio/fomo run ops:refresh-oauth -- --user-id founder
//   pnpm --filter @brevio/fomo run ops:refresh-oauth -- --user-id 25c1a707-...
//
// Refreshing uses the existing refresh_token. Refresh tokens last
// months unless explicitly revoked, so this should "just work" for any
// user whose oauth_tokens row has a non-null refresh_token_ciphertext.
//
// Per the founder's scope discipline: this is an ops bridge, not a
// permanent fix. The polling worker MUST gain auto-refresh in v0.5.3.

import { refreshAccessToken } from '../src/security/oauth/exchange.js';
import { loadProviderConfig } from '../src/security/oauth/providers/index.js';
import { loadCryptoConfig } from '../src/security/token-crypto.js';
import { loadDbClient } from '../src/db/client.js';
import { PostgresTokenStore } from '../src/db/stores/token-postgres.js';

interface ParsedArgs {
  user_id: string;
  provider: string;
}

function parseArgs(argv: readonly string[]): ParsedArgs {
  const out: { user_id?: string; provider?: string } = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const next = argv[i + 1];
    if (a === '--user-id' && typeof next === 'string') {
      out.user_id = next;
      i++;
    } else if (a === '--provider' && typeof next === 'string') {
      out.provider = next;
      i++;
    }
  }
  if (!out.user_id) {
    throw new Error('Missing required --user-id <id> argument');
  }
  return {
    user_id: out.user_id,
    provider: out.provider ?? 'google'
  };
}

async function main(): Promise<void> {
  const args = parseArgs(process.argv.slice(2));

  if (!(process.env.DATABASE_URL ?? '').trim()) {
    process.stderr.write('[ops-refresh-oauth] DATABASE_URL is not set. Source .env first.\n');
    process.exit(2);
  }

  if (args.provider !== 'google') {
    process.stderr.write(
      `[ops-refresh-oauth] Provider '${args.provider}' is not supported. Only 'google' is wired in v0.5.x.\n`
    );
    process.exit(2);
  }
  const providerConfig = loadProviderConfig('google');
  if (!providerConfig) {
    process.stderr.write(
      `[ops-refresh-oauth] Provider 'google' is not configured. ` +
        'Check GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / BREVIO_OAUTH_REDIRECT_URI_GOOGLE.\n'
    );
    process.exit(2);
  }

  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    process.stderr.write(`[ops-refresh-oauth] DB unavailable: ${dbResult.reason}\n`);
    process.exit(1);
  }

  const crypto = loadCryptoConfig();
  const tokenStore = new PostgresTokenStore(dbResult.client, crypto);

  try {
    // 1. Load existing refresh_token + scope list.
    const existingRefresh = await tokenStore.loadRefreshToken(args.user_id, args.provider);
    if (!existingRefresh) {
      process.stderr.write(
        `[ops-refresh-oauth] No refresh_token stored for user_id='${args.user_id}', provider='${args.provider}'. ` +
          'User must re-OAuth from scratch (refresh requires a stored refresh_token from the original grant).\n'
      );
      process.exit(1);
    }
    const tokens = await tokenStore.list(args.user_id);
    const existing = tokens.find((t) => t.provider === args.provider);
    if (!existing) {
      process.stderr.write(`[ops-refresh-oauth] oauth_tokens row not found for ${args.user_id}/${args.provider}.\n`);
      process.exit(1);
    }

    // 2. Call Google's refresh endpoint.
    const result = await refreshAccessToken({
      refreshToken: existingRefresh,
      config: providerConfig
    });

    // 3. Carry forward existing refresh_token if Google didn't return a
    //    new one (typical — Google reissues refresh_token rarely).
    const newRefresh = result.refresh_token ?? existingRefresh;
    const expires_at =
      result.expires_in !== undefined ? new Date(Date.now() + result.expires_in * 1000) : undefined;

    // 4. Save — clears needs_reauth=false as a side effect of save().
    await tokenStore.save({
      user_id: args.user_id,
      provider: args.provider,
      scopes: existing.scopes,
      access_token: result.access_token,
      refresh_token: newRefresh,
      expires_at
    });

    process.stdout.write('\n');
    process.stdout.write('✓ OAuth refresh succeeded.\n');
    process.stdout.write(`  user_id:      ${args.user_id}\n`);
    process.stdout.write(`  provider:     ${args.provider}\n`);
    process.stdout.write(`  expires_at:   ${expires_at?.toISOString() ?? '(no expiry returned)'}\n`);
    process.stdout.write(`  refresh_kept: ${!result.refresh_token ? 'yes (no new refresh_token in response)' : 'new refresh_token issued'}\n`);
    process.stdout.write('\n');
    process.stdout.write('  needs_reauth has been cleared. Polling will resume for this user on the next cycle.\n');
    process.stdout.write('\n');
  } finally {
    await dbResult.pool.end();
  }
}

main().catch((err) => {
  process.stderr.write(`[ops-refresh-oauth] fatal: ${err instanceof Error ? err.message : String(err)}\n`);
  process.exit(1);
});
