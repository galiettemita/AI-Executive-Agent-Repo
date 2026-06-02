// Phase v0.5.3 item #2 — OAuth auto-refresh regression tests.
//
// v0.5.2 surfaced that the polling worker never called refreshAccessToken,
// so every user got locked out 1h after their last manual OAuth.
// These tests codify the contract that closes the gap:
//   - Still-valid token (expires_at > now + skew) → no refresh, no audit
//   - Expired token + valid refresh_token → refresh + save + audit refreshed
//   - Expired token + invalid_grant from Google → mark needs_reauth +
//       audit refresh_failed + return needs_reauth (per founder correction #2)
//   - Expired token + 5xx → return transient_fail, do NOT mark needs_reauth
//   - Expired token + no refresh_token stored → mark needs_reauth + audit
//   - Already-marked needs_reauth + fresh access_token → don't auto-retry
//     (operator must re-OAuth)

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { buildOAuthRefreshHelper } from './refresh-helper.ts';
import { InMemoryTokenStore, type TokenStore } from './token-store.ts';
import type { ProviderConfig } from './providers/index.ts';
import type { CryptoConfig } from '../token-crypto.ts';
import { InMemoryAuditStore } from '../../core/audit.ts';

const TEST_KEK = Buffer.alloc(32, 17);
const cryptoConfig: CryptoConfig = { kek: TEST_KEK, devMode: false };

const providerConfig: ProviderConfig = {
  id: 'google',
  authorizeUrl: 'https://example.com/authorize',
  tokenUrl: 'https://example.com/token',
  revokeUrl: 'https://example.com/revoke',
  clientId: 'cid',
  clientSecret: 'sec',
  redirectUri: 'http://localhost/callback'
};

function mockFetch(
  handler: (url: string, init: RequestInit) => Promise<{ status: number; body: unknown }>
): typeof fetch {
  return (async (url: string | URL, init?: RequestInit) => {
    const result = await handler(url.toString(), init ?? {});
    return new Response(JSON.stringify(result.body), {
      status: result.status,
      headers: { 'content-type': 'application/json' }
    });
  }) as typeof fetch;
}

interface Harness {
  tokenStore: TokenStore;
  auditStore: InMemoryAuditStore;
  now: number;
  setNow: (ms: number) => void;
}

function buildHarness(): Harness {
  const tokenStore = new InMemoryTokenStore(cryptoConfig);
  const auditStore = new InMemoryAuditStore();
  let now = Date.parse('2026-06-01T22:00:00Z');
  return {
    tokenStore,
    auditStore,
    get now() { return now; },
    setNow(ms: number) { now = ms; }
  };
}

describe('buildOAuthRefreshHelper.refreshIfNeeded (Phase v0.5.3 item #2)', () => {
  it('returns still_valid when expires_at is more than skewSeconds in the future', async () => {
    const h = buildHarness();
    // Token expires in 1h, skew is 60s default → still valid.
    await h.tokenStore.save({
      user_id: 'u1',
      provider: 'google',
      scopes: ['gmail.readonly'],
      access_token: 'at_old',
      refresh_token: 'rt_old',
      expires_at: new Date(h.now + 3600 * 1000)
    });
    let fetchCalled = 0;
    const helper = buildOAuthRefreshHelper({
      tokenStore: h.tokenStore,
      auditStore: h.auditStore,
      providerConfig,
      now: () => h.now,
      fetchImpl: mockFetch(async () => { fetchCalled++; return { status: 200, body: {} }; })
    });

    const result = await helper.refreshIfNeeded('u1');
    assert.equal(result.kind, 'still_valid');
    assert.equal(fetchCalled, 0, 'must NOT call Google when token is still valid');
    // No audit row.
    const events = await h.auditStore.recent('u1', 50);
    assert.equal(events.filter((e) => e.action.startsWith('fomo.oauth.')).length, 0);
  });

  it('refreshes + saves + audits when token is expired and refresh_token is valid', async () => {
    const h = buildHarness();
    await h.tokenStore.save({
      user_id: 'u2',
      provider: 'google',
      scopes: ['gmail.readonly'],
      access_token: 'at_old',
      refresh_token: 'rt_valid',
      expires_at: new Date(h.now - 10 * 1000) // expired 10s ago
    });
    const helper = buildOAuthRefreshHelper({
      tokenStore: h.tokenStore,
      auditStore: h.auditStore,
      providerConfig,
      now: () => h.now,
      fetchImpl: mockFetch(async (url, init) => {
        assert.equal(url, providerConfig.tokenUrl);
        assert.match(init.body as string, /grant_type=refresh_token/);
        assert.match(init.body as string, /refresh_token=rt_valid/);
        return {
          status: 200,
          body: { access_token: 'at_new', expires_in: 3600, token_type: 'Bearer' }
        };
      })
    });

    const result = await helper.refreshIfNeeded('u2');
    assert.equal(result.kind, 'refreshed');

    // New access_token saved.
    const newAccess = await h.tokenStore.loadAccessToken('u2', 'google');
    assert.equal(newAccess, 'at_new');

    // refresh_token preserved (Google didn't send a new one in this fixture).
    const oldRefresh = await h.tokenStore.loadRefreshToken('u2', 'google');
    assert.equal(oldRefresh, 'rt_valid');

    // needs_reauth was cleared by save().
    const view = (await h.tokenStore.list('u2')).find((t) => t.provider === 'google');
    assert.equal(view?.needs_reauth, false);

    // Audit row fomo.oauth.refreshed fired.
    const events = await h.auditStore.recent('u2', 50);
    const refreshed = events.find((e) => e.action === 'fomo.oauth.refreshed');
    assert.ok(refreshed, 'expected fomo.oauth.refreshed audit row');
    assert.equal((refreshed?.detail as { provider: string }).provider, 'google');
    // NEVER the access_token plaintext in audit.
    assert.equal(JSON.stringify(refreshed?.detail ?? {}).includes('at_new'), false);
  });

  it('returns needs_reauth + marks token + audits on invalid_grant (revoked refresh_token) — founder correction #2', async () => {
    const h = buildHarness();
    await h.tokenStore.save({
      user_id: 'u3',
      provider: 'google',
      scopes: ['gmail.readonly'],
      access_token: 'at_old',
      refresh_token: 'rt_revoked',
      expires_at: new Date(h.now - 10 * 1000)
    });
    let fetchCalled = 0;
    const helper = buildOAuthRefreshHelper({
      tokenStore: h.tokenStore,
      auditStore: h.auditStore,
      providerConfig,
      now: () => h.now,
      fetchImpl: mockFetch(async () => {
        fetchCalled++;
        return {
          status: 400,
          body: { error: 'invalid_grant', error_description: 'Token has been expired or revoked.' }
        };
      })
    });

    const result = await helper.refreshIfNeeded('u3');
    assert.equal(result.kind, 'needs_reauth');
    assert.equal(fetchCalled, 1);

    // Token row marked needs_reauth=true.
    const view = (await h.tokenStore.list('u3')).find((t) => t.provider === 'google');
    assert.equal(view?.needs_reauth, true);

    // Audit row fomo.oauth.refresh_failed fired with safe detail.
    const events = await h.auditStore.recent('u3', 50);
    const failed = events.find((e) => e.action === 'fomo.oauth.refresh_failed');
    assert.ok(failed, 'expected fomo.oauth.refresh_failed audit row');
    const detail = failed?.detail as { provider: string; reason: string; http_status: number };
    assert.equal(detail.provider, 'google');
    assert.equal(detail.reason, 'invalid_grant');
    assert.equal(detail.http_status, 400);
    // NEVER the refresh_token plaintext or error_description body.
    assert.equal(JSON.stringify(failed?.detail ?? {}).includes('rt_revoked'), false);
    assert.equal(JSON.stringify(failed?.detail ?? {}).includes('expired or revoked'), false);
  });

  it('returns transient_fail on 5xx (does NOT mark needs_reauth — next cycle retries)', async () => {
    const h = buildHarness();
    await h.tokenStore.save({
      user_id: 'u4',
      provider: 'google',
      scopes: ['gmail.readonly'],
      access_token: 'at_old',
      refresh_token: 'rt_valid',
      expires_at: new Date(h.now - 10 * 1000)
    });
    const helper = buildOAuthRefreshHelper({
      tokenStore: h.tokenStore,
      auditStore: h.auditStore,
      providerConfig,
      now: () => h.now,
      fetchImpl: mockFetch(async () => ({
        status: 503,
        body: { error: 'service_unavailable' }
      }))
    });

    const result = await helper.refreshIfNeeded('u4');
    assert.equal(result.kind, 'transient_fail');

    // Critical: needs_reauth must NOT be flipped — refresh_token is
    // still valid; next cycle will retry.
    const view = (await h.tokenStore.list('u4')).find((t) => t.provider === 'google');
    assert.equal(view?.needs_reauth, false);

    // No fomo.oauth.refresh_failed audit (that's reserved for
    // terminal 4xx). No fomo.oauth.refreshed either.
    const events = await h.auditStore.recent('u4', 50);
    assert.equal(events.find((e) => e.action === 'fomo.oauth.refresh_failed'), undefined);
    assert.equal(events.find((e) => e.action === 'fomo.oauth.refreshed'), undefined);
  });

  it('returns needs_reauth + audits when no refresh_token is stored (cannot refresh — must re-OAuth)', async () => {
    const h = buildHarness();
    await h.tokenStore.save({
      user_id: 'u5',
      provider: 'google',
      scopes: ['gmail.readonly'],
      access_token: 'at_old',
      // refresh_token deliberately omitted
      expires_at: new Date(h.now - 10 * 1000)
    });
    let fetchCalled = 0;
    const helper = buildOAuthRefreshHelper({
      tokenStore: h.tokenStore,
      auditStore: h.auditStore,
      providerConfig,
      now: () => h.now,
      fetchImpl: mockFetch(async () => { fetchCalled++; return { status: 200, body: {} }; })
    });

    const result = await helper.refreshIfNeeded('u5');
    assert.equal(result.kind, 'needs_reauth');
    // Google was NEVER called (we don't have a refresh_token to send).
    assert.equal(fetchCalled, 0);

    // Token row marked needs_reauth=true.
    const view = (await h.tokenStore.list('u5')).find((t) => t.provider === 'google');
    assert.equal(view?.needs_reauth, true);

    // Audit fired with the no_refresh_token_stored reason.
    const events = await h.auditStore.recent('u5', 50);
    const failed = events.find((e) => e.action === 'fomo.oauth.refresh_failed');
    assert.ok(failed);
    assert.equal((failed?.detail as { reason: string }).reason, 'no_refresh_token_stored');
  });

  it('does NOT auto-retry a previously-marked needs_reauth token (operator must re-OAuth)', async () => {
    const h = buildHarness();
    // Token still has time, but a prior refresh failed and marked it
    // needs_reauth. We must NOT keep retrying — refresh_token is bad.
    await h.tokenStore.save({
      user_id: 'u6',
      provider: 'google',
      scopes: ['gmail.readonly'],
      access_token: 'at_old',
      refresh_token: 'rt_known_bad',
      expires_at: new Date(h.now + 3600 * 1000) // still valid in time
    });
    await h.tokenStore.markNeedsReauth('u6', 'google');

    let fetchCalled = 0;
    const helper = buildOAuthRefreshHelper({
      tokenStore: h.tokenStore,
      auditStore: h.auditStore,
      providerConfig,
      now: () => h.now,
      fetchImpl: mockFetch(async () => { fetchCalled++; return { status: 200, body: {} }; })
    });

    const result = await helper.refreshIfNeeded('u6');
    assert.equal(result.kind, 'needs_reauth');
    assert.equal((result as { reason: string }).reason, 'previously_marked_needs_reauth');
    // Google must NOT be called (would waste API calls on a known-bad token).
    assert.equal(fetchCalled, 0);
  });
});
