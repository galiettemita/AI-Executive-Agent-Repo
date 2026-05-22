import type { ConsentCategory } from './skill-tiers.js';
import { getDefaultEnabledSkillIds, getSkillsByTier } from './skill-tiers.js';

export interface CapabilityInventoryRecord {
  tenant_id?: string;
  workspace_id?: string;
  user_id?: string;
  device_id?: string;
  enabled_skills: string[];
  denied_skills?: string[];
}

export interface CapabilityIdentity {
  tenantId?: string;
  workspaceId?: string;
  userId?: string;
  deviceId?: string;
}

export interface CapabilityResolution {
  enabledSkills: string[];
  deniedSkills: string[];
  source: 'explicit' | 'inventory' | 'merged' | 'none';
}

function unique(values: string[] | undefined): string[] {
  return [...new Set((values ?? []).map((value) => value.trim()).filter((value) => value.length > 0))];
}

export function parseCapabilityInventory(raw: string | undefined): CapabilityInventoryRecord[] {
  if (!raw || raw.trim().length === 0) {
    return [];
  }

  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      return [];
    }

    return parsed
      .map((item) => {
        if (!item || typeof item !== 'object' || Array.isArray(item)) {
          return undefined;
        }
        const record = item as Record<string, unknown>;
        const enabled_skills = unique(Array.isArray(record.enabled_skills) ? record.enabled_skills.filter((value): value is string => typeof value === 'string') : undefined);
        const denied_skills = unique(Array.isArray(record.denied_skills) ? record.denied_skills.filter((value): value is string => typeof value === 'string') : undefined);
        if (enabled_skills.length === 0 && denied_skills.length === 0) {
          return undefined;
        }
        return {
          tenant_id: typeof record.tenant_id === 'string' && record.tenant_id.trim().length > 0 ? record.tenant_id.trim() : undefined,
          workspace_id: typeof record.workspace_id === 'string' && record.workspace_id.trim().length > 0 ? record.workspace_id.trim() : undefined,
          user_id: typeof record.user_id === 'string' && record.user_id.trim().length > 0 ? record.user_id.trim() : undefined,
          device_id: typeof record.device_id === 'string' && record.device_id.trim().length > 0 ? record.device_id.trim() : undefined,
          enabled_skills,
          denied_skills
        } satisfies CapabilityInventoryRecord;
      })
      .filter((item): item is CapabilityInventoryRecord => Boolean(item));
  } catch {
    return [];
  }
}

export function resolveCapabilityInventory(
  inventory: CapabilityInventoryRecord[],
  identity: CapabilityIdentity,
  explicitEnabledSkills?: string[]
): CapabilityResolution {
  const explicit = unique(explicitEnabledSkills);
  const matches = inventory.filter((record) => {
    const tenantMatch = record.tenant_id ? record.tenant_id === identity.tenantId : true;
    const workspaceMatch = record.workspace_id ? record.workspace_id === identity.workspaceId : true;
    const userMatch = record.user_id ? record.user_id === identity.userId : true;
    const deviceMatch = record.device_id ? record.device_id === identity.deviceId : true;
    return tenantMatch && workspaceMatch && userMatch && deviceMatch;
  });

  const inventoryEnabled = unique(matches.flatMap((record) => record.enabled_skills));
  const inventoryDenied = unique(matches.flatMap((record) => record.denied_skills ?? []));

  if (explicit.length === 0 && inventoryEnabled.length === 0 && inventoryDenied.length === 0) {
    return {
      enabledSkills: [],
      deniedSkills: [],
      source: 'none'
    };
  }

  if (explicit.length === 0) {
    return {
      enabledSkills: inventoryEnabled.filter((skill) => !inventoryDenied.includes(skill)),
      deniedSkills: inventoryDenied,
      source: 'inventory'
    };
  }

  if (inventoryEnabled.length === 0 && inventoryDenied.length === 0) {
    return {
      enabledSkills: explicit,
      deniedSkills: [],
      source: 'explicit'
    };
  }

  const enabledSkills = explicit.filter((skill) => inventoryEnabled.length === 0 || inventoryEnabled.includes(skill)).filter((skill) => !inventoryDenied.includes(skill));
  const deniedSkills = unique([
    ...inventoryDenied,
    ...explicit.filter((skill) => inventoryEnabled.length > 0 && !inventoryEnabled.includes(skill))
  ]);

  return {
    enabledSkills,
    deniedSkills,
    source: 'merged'
  };
}

export type ConsentState = 'granted' | 'revoked' | 'snoozed';

export interface ConsentMap {
  email?: { state: ConsentState; session_id?: string };
  money?: { state: ConsentState; session_id?: string };
  health?: { state: ConsentState; session_id?: string };
}

export interface ResolveEnabledSkillsOptions {
  consent: ConsentMap | undefined;
  currentSessionId?: string;
  inventoryOverride?: string[];
}

export interface UserEnabledSkillsResolution {
  enabledSkills: string[];
  byCategory: {
    email: { state: ConsentState | 'none'; included: boolean };
    money: { state: ConsentState | 'none'; included: boolean };
    health: { state: ConsentState | 'none'; included: boolean };
  };
  source: 'inventory' | 'computed';
}

function isCategoryGrantedForSession(
  entry: { state: ConsentState; session_id?: string } | undefined,
  currentSessionId: string | undefined
): boolean {
  if (!entry) {
    return false;
  }
  if (entry.state === 'granted') {
    return true;
  }
  if (entry.state === 'snoozed' && entry.session_id && currentSessionId && entry.session_id === currentSessionId) {
    return false;
  }
  if (entry.state === 'snoozed') {
    return false;
  }
  return false;
}

export function resolveEnabledSkillsForUser(options: ResolveEnabledSkillsOptions): UserEnabledSkillsResolution {
  if (options.inventoryOverride && options.inventoryOverride.length > 0) {
    return {
      enabledSkills: unique(options.inventoryOverride),
      byCategory: {
        email: { state: options.consent?.email?.state ?? 'none', included: true },
        money: { state: options.consent?.money?.state ?? 'none', included: true },
        health: { state: options.consent?.health?.state ?? 'none', included: true }
      },
      source: 'inventory'
    };
  }

  const defaults = getDefaultEnabledSkillIds();
  const byCategory: UserEnabledSkillsResolution['byCategory'] = {
    email: { state: options.consent?.email?.state ?? 'none', included: false },
    money: { state: options.consent?.money?.state ?? 'none', included: false },
    health: { state: options.consent?.health?.state ?? 'none', included: false }
  };

  const tierSkills: string[] = [];
  if (isCategoryGrantedForSession(options.consent?.email, options.currentSessionId)) {
    tierSkills.push(...getSkillsByTier('email'));
    byCategory.email.included = true;
  }
  if (isCategoryGrantedForSession(options.consent?.money, options.currentSessionId)) {
    tierSkills.push(...getSkillsByTier('money'));
    byCategory.money.included = true;
  }
  if (isCategoryGrantedForSession(options.consent?.health, options.currentSessionId)) {
    tierSkills.push(...getSkillsByTier('health'));
    byCategory.health.included = true;
  }

  return {
    enabledSkills: unique([...defaults, ...tierSkills]),
    byCategory,
    source: 'computed'
  };
}

export function getCategoriesGranted(consent: ConsentMap | undefined, currentSessionId?: string): ConsentCategory[] {
  if (!consent) {
    return [];
  }
  const out: ConsentCategory[] = [];
  if (isCategoryGrantedForSession(consent.email, currentSessionId)) out.push('email');
  if (isCategoryGrantedForSession(consent.money, currentSessionId)) out.push('money');
  if (isCategoryGrantedForSession(consent.health, currentSessionId)) out.push('health');
  return out;
}
