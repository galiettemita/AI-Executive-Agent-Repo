// Phase v0.5.1 — explicit one-time invite-token issuer.
//
// The founder runs this script ONCE per friend to mint a single-use,
// expiring, intent-bound invite token. The plaintext token is printed
// to stdout exactly once + an /onboard URL the founder shares with
// the friend. The plaintext is NEVER persisted, NEVER logged anywhere
// else (including audit_log), and NEVER recoverable.
//
// Founder corrections (see [[multitenant-design-principles]] §5):
//   * Store ONLY token_hash; plaintext NEVER persisted (5a)
//   * NEVER log plaintext token (5b) — audit detail uses an 8-char
//     hash prefix
//   * NEVER log raw phone numbers (5d) — audit detail uses last-4 slug
//   * Token bound to intended_phone_hash (5e) — the /onboard callback
//     verifies the friend's resolved phone matches
//
// Usage:
//   pnpm --filter @brevio/fomo run issue-friend-token --phone +15555550100
//   pnpm --filter @brevio/fomo run issue-friend-token --phone +15555550100 --ttl-hours 6
//   pnpm --filter @brevio/fomo run issue-friend-token --phone +15555550100 --base-url https://my-ngrok.ngrok.app
//
// The friend opens the printed /onboard URL, completes Google OAuth,
// and the runtime atomically consumes the token AFTER the user
// creation succeeds. See [src/routes/onboard.ts] (Phase v0.5.1 step #4).

import {
  encryptInviteBoundPhone,
  hashPhone,
  loadPhoneHashConfig,
  normalizeE164,
  phoneSlug
} from '../src/security/phone-allowlist.js';
import { PostgresInviteTokenStore, tokenHashPrefix } from '../src/security/invite-tokens.js';
import { loadCryptoConfig } from '../src/security/token-crypto.js';
import { loadDbClient } from '../src/db/client.js';
import { PostgresAuditStore } from '../src/db/stores/audit-postgres.js';

interface ParsedArgs {
  phone: string;
  ttl_hours: number;
  base_url: string;
}

function parseArgs(argv: readonly string[]): ParsedArgs {
  const out: { phone?: string; ttl_hours?: number; base_url?: string } = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const next = argv[i + 1];
    if (a === '--phone' && typeof next === 'string') {
      out.phone = next;
      i++;
    } else if (a === '--ttl-hours' && typeof next === 'string') {
      const n = Number(next);
      if (!Number.isFinite(n) || n <= 0 || n > 168) {
        throw new Error('--ttl-hours must be a positive number ≤ 168 (one week ceiling)');
      }
      out.ttl_hours = n;
      i++;
    } else if (a === '--base-url' && typeof next === 'string') {
      out.base_url = next;
      i++;
    }
  }
  if (!out.phone) {
    throw new Error('Missing required --phone <E.164> argument');
  }
  const baseUrl =
    out.base_url ??
    (process.env.FOMO_FRIEND_BETA_BASE_URL ?? '').trim() ??
    '';
  return {
    phone: out.phone,
    ttl_hours: out.ttl_hours ?? 24,
    base_url: baseUrl
  };
}

async function main(): Promise<void> {
  const args = parseArgs(process.argv.slice(2));

  if (!(process.env.DATABASE_URL ?? '').trim()) {
    process.stderr.write(
      '[issue-friend-token] DATABASE_URL is not set. Source apps/fomo/.env.<env>.local first.\n'
    );
    process.exit(2);
  }

  // Per founder directive: kill switch gates token issue. Founder must
  // explicitly opt-in to friend-beta mode before issuing invites.
  if ((process.env.FOMO_FRIEND_BETA_ENABLED ?? '').trim() !== 'true') {
    process.stderr.write(
      '[issue-friend-token] FOMO_FRIEND_BETA_ENABLED is not "true".\n' +
        '  Friend-beta is gated. Set FOMO_FRIEND_BETA_ENABLED=true to enable token issue.\n'
    );
    process.exit(2);
  }

  const founderUserId = (process.env.FOMO_FOUNDER_USER_ID ?? '').trim();
  if (!founderUserId) {
    process.stderr.write(
      '[issue-friend-token] FOMO_FOUNDER_USER_ID is required (the issuing user_id for audit).\n'
    );
    process.exit(2);
  }

  // Normalize + hash the phone. The plaintext stays in this process
  // memory only — never persisted, never logged.
  let normalizedPhone: string;
  try {
    normalizedPhone = normalizeE164(args.phone);
  } catch {
    process.stderr.write(
      `[issue-friend-token] Invalid --phone argument. Must be E.164 (e.g. +15555550100).\n`
    );
    process.exit(2);
  }

  const phoneHashCfg = loadPhoneHashConfig(process.env);
  const cryptoCfg = loadCryptoConfig();
  const intended_phone_hash = hashPhone(normalizedPhone, phoneHashCfg);
  const intended_phone_encrypted = encryptInviteBoundPhone(
    normalizedPhone,
    intended_phone_hash,
    cryptoCfg
  );
  const slug = phoneSlug(normalizedPhone);

  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    process.stderr.write(`[issue-friend-token] DB unavailable: ${dbResult.reason}\n`);
    process.exit(1);
  }

  const inviteStore = new PostgresInviteTokenStore(dbResult.client);
  const auditStore = new PostgresAuditStore(dbResult.client);

  try {
    const issued = await inviteStore.issue({
      intended_phone_hash,
      intended_phone_encrypted,
      issued_by_user_id: founderUserId,
      ttl_ms: args.ttl_hours * 60 * 60 * 1000
    });

    // Audit on issue. Detail uses ONLY safe identifiers: an 8-char
    // hash prefix for the token, the last-4 phone slug, and the
    // expires_at ISO. NEVER the plaintext token, NEVER the raw phone.
    await auditStore.write({
      actor_user_id: founderUserId,
      actor_ip: null,
      actor_user_agent: 'issue-friend-token-script',
      action: 'fomo.onboard.invite_issued',
      target: `invite_token:${issued.id}`,
      result: 'success',
      detail: {
        invite_id: issued.id,
        token_hash_prefix: tokenHashPrefix(issued.token_hash),
        intended_phone_slug: slug,
        expires_at: issued.expires_at.toISOString(),
        ttl_hours: args.ttl_hours
      }
    });

    // Print the operator-facing output. Plaintext token appears
    // EXACTLY ONCE on stdout; never written to any log file.
    process.stdout.write('\n');
    process.stdout.write('✓ Invite token issued.\n');
    process.stdout.write(`  invite_id:        ${issued.id}\n`);
    process.stdout.write(`  intended_phone:   ...${slug}\n`);
    process.stdout.write(`  expires_at:       ${issued.expires_at.toISOString()}\n`);
    process.stdout.write(`  token_plaintext:  ${issued.token_plaintext}\n`);
    process.stdout.write('\n');
    if (args.base_url) {
      const url = `${args.base_url.replace(/\/+$/, '')}/onboard?token=${issued.token_plaintext}`;
      process.stdout.write(`Share this URL with the friend (one-time, expires in ${args.ttl_hours}h):\n`);
      process.stdout.write(`  ${url}\n`);
    } else {
      process.stdout.write(
        'No --base-url / FOMO_FRIEND_BETA_BASE_URL provided. Build the URL yourself:\n' +
          `  <YOUR_BASE_URL>/onboard?token=${issued.token_plaintext}\n`
      );
    }
    process.stdout.write('\n');
    process.stdout.write(
      'The plaintext token above is NOT recoverable from the database.\n' +
        'If you lose it, issue a new one (this one is harmless — it cannot be reused).\n'
    );
    process.stdout.write('\n');
  } finally {
    await dbResult.pool.end();
  }
}

main().catch((err) => {
  process.stderr.write(`[issue-friend-token] fatal: ${err instanceof Error ? err.message : String(err)}\n`);
  process.exit(1);
});
