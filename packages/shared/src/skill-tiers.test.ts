import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  SKILL_TIER_REGISTRY,
  getCategoryForSkill,
  getDefaultEnabledSkillIds,
  getOAuthProviderForSkill,
  getOAuthScopesForSkill,
  getProvidersForCategory,
  getSkillTier,
  getSkillsByTier,
  getSkillsForProvider,
  isKnownTierSkill
} from './skill-tiers.ts';

describe('skill-tiers registry', () => {
  it('every entry has a non-empty tier', () => {
    for (const [skill_id, entry] of Object.entries(SKILL_TIER_REGISTRY)) {
      assert.ok(entry.tier, `entry ${skill_id} missing tier`);
      assert.ok(['safe', 'email', 'money', 'health', 'dangerous'].includes(entry.tier), `entry ${skill_id} has invalid tier ${entry.tier}`);
    }
  });

  it('default-enabled skills are the safe-tier skills', () => {
    const safe = getSkillsByTier('safe');
    const defaults = getDefaultEnabledSkillIds();
    assert.deepEqual(defaults.slice().sort(), safe.slice().sort());
  });

  it('default-enabled count matches expected baseline (>= 35 safe skills)', () => {
    const defaults = getDefaultEnabledSkillIds();
    assert.ok(defaults.length >= 35, `expected at least 35 safe skills, got ${defaults.length}`);
  });

  it('email tier is non-empty', () => {
    assert.ok(getSkillsByTier('email').length > 0);
  });

  it('every entry with oauth_scopes also declares oauth_provider', () => {
    for (const [skill_id, entry] of Object.entries(SKILL_TIER_REGISTRY)) {
      if (entry.oauth_scopes && entry.oauth_scopes.length > 0) {
        assert.ok(entry.oauth_provider, `${skill_id} declares oauth_scopes but no oauth_provider`);
      }
    }
  });

  it('getSkillTier returns expected values for known and unknown skills', () => {
    assert.equal(getSkillTier('google-calendar'), 'safe');
    assert.equal(getSkillTier('google-workspace'), 'email');
    assert.equal(getSkillTier('ynab'), 'money');
    assert.equal(getSkillTier('healthkit-sync-apple'), 'health');
    assert.equal(getSkillTier('totally-fake-skill'), undefined);
  });

  it('getCategoryForSkill returns category only for sensitive tiers', () => {
    assert.equal(getCategoryForSkill('google-workspace'), 'email');
    assert.equal(getCategoryForSkill('ynab'), 'money');
    assert.equal(getCategoryForSkill('healthkit-sync-apple'), 'health');
    assert.equal(getCategoryForSkill('google-calendar'), undefined);
    assert.equal(getCategoryForSkill('todoist'), undefined);
  });

  it('getProvidersForCategory returns google+microsoft for email', () => {
    const providers = getProvidersForCategory('email');
    assert.ok(providers.includes('google'));
    assert.ok(providers.includes('microsoft'));
  });

  it('getOAuthProviderForSkill returns the provider when present', () => {
    assert.equal(getOAuthProviderForSkill('google-calendar'), 'google');
    assert.equal(getOAuthProviderForSkill('outlook'), 'microsoft');
    assert.equal(getOAuthProviderForSkill('todoist'), undefined);
  });

  it('getSkillsForProvider returns all skills under that provider', () => {
    const googleSkills = getSkillsForProvider('google');
    assert.ok(googleSkills.includes('google-calendar'));
    assert.ok(googleSkills.includes('google-workspace'));
    assert.ok(googleSkills.includes('gkeep'));
  });

  it('getOAuthScopesForSkill returns scopes for OAuth skills, empty for others', () => {
    assert.deepEqual(getOAuthScopesForSkill('google-calendar'), ['https://www.googleapis.com/auth/calendar.events']);
    assert.deepEqual(getOAuthScopesForSkill('todoist'), []);
    assert.deepEqual(getOAuthScopesForSkill('totally-fake-skill'), []);
  });

  it('isKnownTierSkill returns true for catalog skills, false for others', () => {
    assert.equal(isKnownTierSkill('google-calendar'), true);
    assert.equal(isKnownTierSkill('totally-fake-skill'), false);
  });
});
