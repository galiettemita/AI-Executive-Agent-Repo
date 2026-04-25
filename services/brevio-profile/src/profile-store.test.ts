import assert from 'node:assert/strict';
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { tmpdir } from 'node:os';
import { describe, it } from 'node:test';

import { ProfileStore } from './profile-store.js';

function testProfilesRoot(): string {
  return mkdtempSync(path.join(tmpdir(), 'brevio-profile-'));
}

describe('ProfileStore', () => {
  it('does not mutate updated_at when a profile is read repeatedly', async () => {
    const store = new ProfileStore(testProfilesRoot());

    const first = await store.ensureProfile('user-12345');
    const second = await store.ensureProfile('user-12345');

    assert.equal(second.updated_at, first.updated_at);
    assert.equal(second.profile_hash, first.profile_hash);
  });

  it('includes timezone, locale, and preferences in the profile hash', async () => {
    const store = new ProfileStore(testProfilesRoot());

    const first = await store.ensureProfile('user-12345');
    const updated = await store.updatePreferences('user-12345', {
      preferences: { theme: 'warm', digest: true },
      timezone: 'America/New_York',
      locale: 'en-GB'
    });

    assert.notEqual(updated.profile_hash, first.profile_hash);
    assert.equal(updated.timezone, 'America/New_York');
    assert.equal(updated.locale, 'en-GB');
    assert.deepEqual(updated.preferences, { theme: 'warm', digest: true });
  });

  it('updates the profile hash when knowledge content changes', async () => {
    const store = new ProfileStore(testProfilesRoot());

    const first = await store.ensureProfile('user-12345');
    const updated = await store.writeKnowledge('user-12345', 'USER.md', 'Prefers concise summaries.');

    assert.notEqual(updated.profile_hash, first.profile_hash);
  });

  it('fails fast on corrupt profile metadata', async () => {
    const root = testProfilesRoot();
    const userDir = path.join(root, 'user-12345');
    mkdirSync(userDir, { recursive: true });
    writeFileSync(path.join(userDir, 'profile.json'), '{"user_id":"user-12345","preferences":[]}', { encoding: 'utf8', flag: 'w' });

    const store = new ProfileStore(root);
    await assert.rejects(() => store.ensureProfile('user-12345'), /profile .* invalid|profile file must contain a JSON object/);
  });
});
