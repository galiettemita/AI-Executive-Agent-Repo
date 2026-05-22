import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  getCategoriesGranted,
  parseCapabilityInventory,
  resolveCapabilityInventory,
  resolveEnabledSkillsForUser
} from './capability-inventory.ts';
import { getDefaultEnabledSkillIds, getSkillsByTier } from './skill-tiers.ts';

describe('capability inventory', () => {
  it('parses inventory records safely', () => {
    const inventory = parseCapabilityInventory(
      JSON.stringify([
        {
          user_id: 'user-1',
          enabled_skills: ['todoist', 'apple-mail'],
          denied_skills: ['doing-tasks']
        }
      ])
    );

    assert.equal(inventory.length, 1);
    assert.deepEqual(inventory[0]?.enabled_skills, ['todoist', 'apple-mail']);
  });

  it('merges explicit skills with inventory constraints', () => {
    const inventory = parseCapabilityInventory(
      JSON.stringify([
        {
          user_id: 'user-1',
          enabled_skills: ['todoist'],
          denied_skills: ['doing-tasks']
        }
      ])
    );

    const resolution = resolveCapabilityInventory(inventory, { userId: 'user-1' }, ['todoist', 'doing-tasks']);

    assert.deepEqual(resolution.enabledSkills, ['todoist']);
    assert.deepEqual(resolution.deniedSkills, ['doing-tasks']);
    assert.equal(resolution.source, 'merged');
  });

  it('matches scoped records across tenant workspace user and device identity', () => {
    const inventory = parseCapabilityInventory(
      JSON.stringify([
        {
          tenant_id: 'tenant-1',
          workspace_id: 'workspace-1',
          user_id: 'user-1',
          device_id: 'device-1',
          enabled_skills: ['voice-wake-say']
        },
        {
          tenant_id: 'tenant-1',
          workspace_id: 'workspace-1',
          user_id: 'user-1',
          enabled_skills: ['todoist']
        }
      ])
    );

    const resolution = resolveCapabilityInventory(inventory, {
      tenantId: 'tenant-1',
      workspaceId: 'workspace-1',
      userId: 'user-1',
      deviceId: 'device-1'
    });

    assert.deepEqual(resolution.enabledSkills.sort(), ['todoist', 'voice-wake-say']);
    assert.equal(resolution.source, 'inventory');
  });
});

describe('resolveEnabledSkillsForUser', () => {
  it('returns only safe-tier skills when no consent is granted', () => {
    const result = resolveEnabledSkillsForUser({ consent: undefined });
    const defaults = getDefaultEnabledSkillIds();
    assert.equal(result.source, 'computed');
    assert.deepEqual(result.enabledSkills.slice().sort(), defaults.slice().sort());
    assert.equal(result.byCategory.email.included, false);
    assert.equal(result.byCategory.money.included, false);
    assert.equal(result.byCategory.health.included, false);
  });

  it('adds email-tier skills when email consent is granted', () => {
    const result = resolveEnabledSkillsForUser({
      consent: { email: { state: 'granted' } }
    });
    const emailSkills = getSkillsByTier('email');
    for (const skill of emailSkills) {
      assert.ok(result.enabledSkills.includes(skill), `expected ${skill} to be enabled`);
    }
    assert.equal(result.byCategory.email.included, true);
    assert.equal(result.byCategory.money.included, false);
  });

  it('does not include money-tier skills when money is snoozed for current session', () => {
    const result = resolveEnabledSkillsForUser({
      consent: { money: { state: 'snoozed', session_id: 'sess-A' } },
      currentSessionId: 'sess-A'
    });
    for (const skill of getSkillsByTier('money')) {
      assert.ok(!result.enabledSkills.includes(skill), `expected ${skill} NOT to be enabled (snoozed)`);
    }
    assert.equal(result.byCategory.money.state, 'snoozed');
    assert.equal(result.byCategory.money.included, false);
  });

  it('honors inventoryOverride when provided', () => {
    const result = resolveEnabledSkillsForUser({
      consent: { email: { state: 'granted' } },
      inventoryOverride: ['some-ops-skill']
    });
    assert.deepEqual(result.enabledSkills, ['some-ops-skill']);
    assert.equal(result.source, 'inventory');
  });

  it('rejects revoked categories even if other categories are granted', () => {
    const result = resolveEnabledSkillsForUser({
      consent: {
        email: { state: 'granted' },
        money: { state: 'revoked' }
      }
    });
    for (const skill of getSkillsByTier('money')) {
      assert.ok(!result.enabledSkills.includes(skill), `expected ${skill} NOT to be enabled (revoked)`);
    }
    assert.equal(result.byCategory.money.included, false);
    assert.equal(result.byCategory.email.included, true);
  });
});

describe('getCategoriesGranted', () => {
  it('returns granted categories only', () => {
    const out = getCategoriesGranted({
      email: { state: 'granted' },
      money: { state: 'revoked' },
      health: { state: 'snoozed', session_id: 'X' }
    }, 'X');
    assert.deepEqual(out, ['email']);
  });

  it('returns empty when no consent', () => {
    assert.deepEqual(getCategoriesGranted(undefined), []);
  });
});
