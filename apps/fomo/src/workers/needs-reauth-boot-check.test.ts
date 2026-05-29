// Phase 3G.1 item #3 — regression test for boot-time needs_reauth
// visibility.
//
// Original incident: 2026-05-28 UTC. The polling worker silently
// skipped the founder for 18+ hours because needs_reauth=true.
// Discovered only via psql query. Founder directive 2026-05-29: must
// use the SAME active-user set the polling worker uses (cursorStore
// .listUserIds()), not a broader oauth_tokens scan that could
// include orphan token rows.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryGmailCursorStore } from '../memory/gmail-cursors.js';
import { loadCryptoConfig } from '../security/token-crypto.js';
import { InMemoryTokenStore } from '../security/oauth/token-store.js';
import { findUsersNeedingReauth } from './needs-reauth-boot-check.js';

const TEST_KEK = Buffer.alloc(32, 13).toString('base64');

function withEnv<T>(overrides: Record<string, string | undefined>, fn: () => T): T {
  const prev: Record<string, string | undefined> = {};
  for (const [k, v] of Object.entries(overrides)) {
    prev[k] = process.env[k];
    if (v === undefined) delete process.env[k];
    else process.env[k] = v;
  }
  try {
    return fn();
  } finally {
    for (const [k, v] of Object.entries(prev)) {
      if (v === undefined) delete process.env[k];
      else process.env[k] = v;
    }
  }
}

const cryptoConfig = withEnv(
  { BREVIO_TOKEN_KEK: TEST_KEK, BREVIO_DEV_MODE: undefined },
  () => loadCryptoConfig()
);

async function makeStores(): Promise<{ cursorStore: InMemoryGmailCursorStore; tokenStore: InMemoryTokenStore }> {
  const cursorStore = new InMemoryGmailCursorStore();
  const tokenStore = new InMemoryTokenStore(cryptoConfig);
  return { cursorStore, tokenStore };
}

describe('findUsersNeedingReauth (Phase 3G.1 item #3)', () => {
  it('returns empty when no cursors exist (in-memory dev, no users connected)', async () => {
    const { cursorStore, tokenStore } = await makeStores();
    const findings = await findUsersNeedingReauth({ cursorStore, tokenStore });
    assert.deepEqual(findings, []);
  });

  it('returns empty when the active user has a fresh token (needs_reauth=false)', async () => {
    const { cursorStore, tokenStore } = await makeStores();
    await cursorStore.upsert({ user_id: 'founder', history_id: 'h-1' });
    await tokenStore.save({
      user_id: 'founder',
      provider: 'google',
      access_token: 'at',
      refresh_token: 'rt',
      scopes: ['gmail.readonly'],
      expires_at: new Date(Date.now() + 3600_000)
    });
    const findings = await findUsersNeedingReauth({ cursorStore, tokenStore });
    assert.deepEqual(findings, []);
  });

  it('returns one finding when the active user has needs_reauth=true (incident reproduction)', async () => {
    const { cursorStore, tokenStore } = await makeStores();
    await cursorStore.upsert({ user_id: 'founder', history_id: 'h-1' });
    await tokenStore.save({
      user_id: 'founder',
      provider: 'google',
      access_token: 'at',
      refresh_token: 'rt',
      scopes: ['gmail.readonly'],
      expires_at: new Date(Date.now() + 3600_000)
    });
    await tokenStore.markNeedsReauth('founder', 'google');
    const findings = await findUsersNeedingReauth({ cursorStore, tokenStore });
    assert.equal(findings.length, 1);
    assert.equal(findings[0].user_id, 'founder');
    assert.equal(findings[0].provider, 'google');
  });

  it('uses the cursorStore active-user set, not a broader token scan (founder directive)', async () => {
    // Token row exists for an "orphan" user with no cursor. The
    // polling worker would never touch them. The boot WARN should
    // not surface them either, since the founder directive scopes
    // visibility to the same active-user set.
    const { cursorStore, tokenStore } = await makeStores();
    await tokenStore.save({
      user_id: 'orphan-with-stale-token',
      provider: 'google',
      access_token: 'at',
      refresh_token: 'rt',
      scopes: ['gmail.readonly'],
      expires_at: new Date(Date.now() + 3600_000)
    });
    await tokenStore.markNeedsReauth('orphan-with-stale-token', 'google');

    const findings = await findUsersNeedingReauth({ cursorStore, tokenStore });
    assert.deepEqual(
      findings,
      [],
      'orphan token row (no cursor → not in polling worker active set) must NOT be surfaced'
    );
  });

  it('surfaces every user with needs_reauth=true when multiple cursors exist', async () => {
    const { cursorStore, tokenStore } = await makeStores();
    for (const uid of ['founder', 'beta-friend-1', 'beta-friend-2']) {
      await cursorStore.upsert({ user_id: uid, history_id: `h-${uid}` });
      await tokenStore.save({
        user_id: uid,
        provider: 'google',
        access_token: 'at',
        refresh_token: 'rt',
        scopes: ['gmail.readonly'],
        expires_at: new Date(Date.now() + 3600_000)
      });
    }
    await tokenStore.markNeedsReauth('beta-friend-1', 'google');
    await tokenStore.markNeedsReauth('beta-friend-2', 'google');
    const findings = await findUsersNeedingReauth({ cursorStore, tokenStore });
    assert.equal(findings.length, 2);
    const uids = findings.map((f) => f.user_id).sort();
    assert.deepEqual(uids, ['beta-friend-1', 'beta-friend-2']);
  });

  it('return value is frozen so callers cannot mutate the surfaced findings', async () => {
    const { cursorStore, tokenStore } = await makeStores();
    await cursorStore.upsert({ user_id: 'founder', history_id: 'h-1' });
    await tokenStore.save({
      user_id: 'founder',
      provider: 'google',
      access_token: 'at',
      refresh_token: 'rt',
      scopes: ['gmail.readonly'],
      expires_at: new Date(Date.now() + 3600_000)
    });
    await tokenStore.markNeedsReauth('founder', 'google');
    const findings = await findUsersNeedingReauth({ cursorStore, tokenStore });
    assert.ok(Object.isFrozen(findings));
  });
});
