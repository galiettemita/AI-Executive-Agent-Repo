// ConsentStore interface + InMemoryConsentStore implementation.
//
// In dev mode (BREVIO_DEV_MODE=true) the gateway uses InMemoryConsentStore so the
// simulator runs without a Postgres dependency. Production uses PostgresConsentStore
// (deferred to Block H — schema migration 012 is already in place).

import {
  type ConsentCategory,
  type ConsentMap,
  type ConsentState,
  type UserEnabledSkillsResolution,
  resolveEnabledSkillsForUser
} from '@brevio/shared';

export interface ConsentRow {
  category: ConsentCategory;
  state: ConsentState;
  source: 'inline_prompt' | 'settings' | 'oauth_callback' | 'api' | 'admin';
  granted_at?: string;
  revoked_at?: string;
  snoozed_until_session?: string;
  updated_at: string;
}

export interface SetConsentInput {
  user_id: string;
  category: ConsentCategory;
  state: ConsentState;
  source: ConsentRow['source'];
  session_id?: string;
}

export interface RevokeOutcome {
  consent_state: 'revoked';
  providers_to_disconnect: string[];
}

export interface ConsentStore {
  getCategoryStates(userId: string): Promise<ConsentMap>;
  setCategoryState(input: SetConsentInput): Promise<void>;
  revokeCategory(userId: string, category: ConsentCategory): Promise<RevokeOutcome>;
  markOnboardingDismissed(userId: string, ts?: string): Promise<void>;
  getOnboardingDismissed(userId: string): Promise<string | null>;
  computeEnabledSkills(userId: string, currentSessionId?: string): Promise<UserEnabledSkillsResolution>;
}

export class InMemoryConsentStore implements ConsentStore {
  private readonly consent = new Map<string, Map<ConsentCategory, ConsentRow>>();
  private readonly onboarding = new Map<string, string>();

  async getCategoryStates(userId: string): Promise<ConsentMap> {
    const userMap = this.consent.get(userId);
    if (!userMap) return {};
    const out: ConsentMap = {};
    for (const [category, row] of userMap.entries()) {
      out[category] = { state: row.state, session_id: row.snoozed_until_session };
    }
    return out;
  }

  async setCategoryState(input: SetConsentInput): Promise<void> {
    let userMap = this.consent.get(input.user_id);
    if (!userMap) {
      userMap = new Map();
      this.consent.set(input.user_id, userMap);
    }
    const now = new Date().toISOString();
    const row: ConsentRow = {
      category: input.category,
      state: input.state,
      source: input.source,
      updated_at: now
    };
    if (input.state === 'granted') row.granted_at = now;
    if (input.state === 'revoked') row.revoked_at = now;
    if (input.state === 'snoozed') row.snoozed_until_session = input.session_id;
    userMap.set(input.category, row);
  }

  async revokeCategory(userId: string, category: ConsentCategory): Promise<RevokeOutcome> {
    await this.setCategoryState({
      user_id: userId,
      category,
      state: 'revoked',
      source: 'settings'
    });
    const providers = providersForCategory(category);
    return { consent_state: 'revoked', providers_to_disconnect: providers };
  }

  async markOnboardingDismissed(userId: string, ts?: string): Promise<void> {
    if (this.onboarding.has(userId)) return;
    this.onboarding.set(userId, ts ?? new Date().toISOString());
  }

  async getOnboardingDismissed(userId: string): Promise<string | null> {
    return this.onboarding.get(userId) ?? null;
  }

  async computeEnabledSkills(userId: string, currentSessionId?: string): Promise<UserEnabledSkillsResolution> {
    const consent = await this.getCategoryStates(userId);
    return resolveEnabledSkillsForUser({ consent, currentSessionId });
  }
}

function providersForCategory(category: ConsentCategory): string[] {
  if (category === 'email') return ['google', 'microsoft'];
  return [];
}

export function createConsentStore(): ConsentStore {
  return new InMemoryConsentStore();
}
