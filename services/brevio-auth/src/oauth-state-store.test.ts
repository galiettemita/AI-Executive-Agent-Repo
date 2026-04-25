import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { tmpdir } from 'node:os';
import { describe, it } from 'node:test';

import { OAuthStateStore } from './oauth-state-store.js';

function testStatePath(name: string): string {
  return path.join(mkdtempSync(path.join(tmpdir(), 'brevio-auth-')), `${name}.json`);
}

describe('OAuthStateStore', () => {
  it('persists oauth states and consumes them once', () => {
    const statePath = testStatePath('oauth-state');
    const createdAtMs = Date.UTC(2026, 3, 9, 12, 0, 0);

    const store = new OAuthStateStore(statePath);
    store.put('state-1', {
      service: 'google',
      userId: 'user-1',
      completionRedirectUri: 'https://example.com/callback',
      codeVerifier: 'verifier-1',
      createdAtMs,
      expiresAtMs: createdAtMs + 60_000
    });

    const reloaded = new OAuthStateStore(statePath);
    assert.equal(reloaded.size(), 1);
    assert.equal(reloaded.get('state-1')?.service, 'google');

    const consumed = reloaded.consume('google', 'state-1', createdAtMs + 1_000);
    assert.equal(consumed?.userId, 'user-1');
    assert.equal(new OAuthStateStore(statePath).size(), 0);
  });

  it('expires persisted oauth states', () => {
    const statePath = testStatePath('oauth-expire');
    const store = new OAuthStateStore(statePath);
    store.put('expired', {
      service: 'google',
      userId: 'user-1',
      completionRedirectUri: 'https://example.com/callback',
      codeVerifier: 'verifier-1',
      createdAtMs: 100,
      expiresAtMs: 200
    });

    store.expire(500);
    assert.equal(new OAuthStateStore(statePath).size(), 0);
  });

  it('fails fast when the persisted oauth snapshot is corrupt', () => {
    const statePath = testStatePath('oauth-corrupt');
    writeFileSync(statePath, JSON.stringify({ version: 1, records: [['state-1', { service: '' }]] }), 'utf8');

    assert.throws(() => new OAuthStateStore(statePath), /oauth state snapshot is corrupt/);
  });
});
