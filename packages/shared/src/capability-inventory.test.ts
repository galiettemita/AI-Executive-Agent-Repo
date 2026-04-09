import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { parseCapabilityInventory, resolveCapabilityInventory } from './capability-inventory.ts';

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
