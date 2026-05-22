// Single source of truth for skill safety tiers.
// Brain catalog (services/brevio-brain/src/catalog.ts) reuses this for tier metadata
// and adds operational fields (operations, deployment_modes, aliases, etc.).
// Gateway (services/brevio-gateway/src/consent-store.ts) reads this directly to
// resolve default-enabled skills and category-provider mappings.

export type SafetyTier = 'safe' | 'email' | 'money' | 'health' | 'dangerous';
export type OAuthProvider = 'google' | 'microsoft' | 'apple' | 'spotify' | 'github' | 'notion';
export type ConsentCategory = 'email' | 'money' | 'health';

export interface SkillTierEntry {
  tier: SafetyTier;
  oauth_provider?: OAuthProvider;
  oauth_scopes?: string[];
}

export const SKILL_TIER_REGISTRY: Record<string, SkillTierEntry> = {
  'aerobase-skill': { tier: 'safe' },
  'apple-mail': { tier: 'email' },
  'apple-mail-search': { tier: 'email' },
  'apple-music': { tier: 'safe' },
  'apple-notes-skill': { tier: 'safe' },
  'asana': { tier: 'safe' },
  'aviationstack-flight-tracker': { tier: 'safe' },
  'bear-notes': { tier: 'safe' },
  'better-notion': { tier: 'safe', oauth_provider: 'notion', oauth_scopes: ['read_content', 'update_content'] },
  'brave-search': { tier: 'safe' },
  'clickup-mcp': { tier: 'safe' },
  'craft': { tier: 'safe' },
  'firecrawl-search': { tier: 'safe' },
  'flight-tracker': { tier: 'safe' },
  'google-calendar': { tier: 'safe', oauth_provider: 'google', oauth_scopes: ['https://www.googleapis.com/auth/calendar.events'] },
  'google-maps': { tier: 'safe' },
  'google-workspace': { tier: 'email', oauth_provider: 'google', oauth_scopes: ['https://www.googleapis.com/auth/gmail.modify', 'https://www.googleapis.com/auth/gmail.send'] },
  'goplaces': { tier: 'safe' },
  'gkeep': { tier: 'safe', oauth_provider: 'google', oauth_scopes: ['https://www.googleapis.com/auth/keep'] },
  'healthkit-sync-apple': { tier: 'health' },
  'imap-email': { tier: 'email' },
  'jira': { tier: 'safe' },
  'linear': { tier: 'safe' },
  'local-places': { tier: 'safe' },
  'obsidian': { tier: 'safe' },
  'omnifocus': { tier: 'safe' },
  'outlook': { tier: 'email', oauth_provider: 'microsoft', oauth_scopes: ['Mail.Read', 'Mail.Send', 'Calendars.Read'] },
  'parcel-package-tracking': { tier: 'safe' },
  'post-at': { tier: 'safe' },
  'reflect': { tier: 'safe' },
  'shopping-expert': { tier: 'safe' },
  'smart-expense-tracker': { tier: 'money' },
  'smtp-send': { tier: 'email' },
  'spotify': { tier: 'safe', oauth_provider: 'spotify', oauth_scopes: ['user-modify-playback-state', 'user-read-playback-state'] },
  'spotify-history': { tier: 'safe', oauth_provider: 'spotify', oauth_scopes: ['user-read-recently-played', 'user-top-read'] },
  'spotify-player': { tier: 'safe', oauth_provider: 'spotify', oauth_scopes: ['user-modify-playback-state'] },
  'spotify-web-api': { tier: 'safe', oauth_provider: 'spotify', oauth_scopes: ['user-modify-playback-state', 'user-read-playback-state'] },
  'spots': { tier: 'safe' },
  'tavily': { tier: 'safe' },
  'things-mac': { tier: 'safe' },
  'thinking-partner': { tier: 'safe' },
  'ticktick': { tier: 'safe' },
  'todoist': { tier: 'safe' },
  'track17': { tier: 'safe' },
  'trello': { tier: 'safe' },
  'watch-my-money': { tier: 'money' },
  'ynab': { tier: 'money' },
  'youtube-api': { tier: 'safe' },
  'youtube-summarizer': { tier: 'safe' },
  'video-transcript-downloader': { tier: 'safe' },
  'ytmusic': { tier: 'safe' }
};

const CATEGORY_PROVIDERS: Record<ConsentCategory, OAuthProvider[]> = {
  email: ['google', 'microsoft'],
  money: [],
  health: []
};

export function getSkillTier(skillId: string): SafetyTier | undefined {
  return SKILL_TIER_REGISTRY[skillId]?.tier;
}

export function getSkillsByTier(tier: SafetyTier): string[] {
  return Object.entries(SKILL_TIER_REGISTRY)
    .filter(([, entry]) => entry.tier === tier)
    .map(([skill_id]) => skill_id);
}

export function getDefaultEnabledSkillIds(): string[] {
  return getSkillsByTier('safe');
}

export function getSkillsForProvider(provider: OAuthProvider): string[] {
  return Object.entries(SKILL_TIER_REGISTRY)
    .filter(([, entry]) => entry.oauth_provider === provider)
    .map(([skill_id]) => skill_id);
}

export function getOAuthScopesForSkill(skillId: string): string[] {
  return SKILL_TIER_REGISTRY[skillId]?.oauth_scopes ?? [];
}

export function getOAuthProviderForSkill(skillId: string): OAuthProvider | undefined {
  return SKILL_TIER_REGISTRY[skillId]?.oauth_provider;
}

export function getProvidersForCategory(category: ConsentCategory): OAuthProvider[] {
  return [...CATEGORY_PROVIDERS[category]];
}

export function getCategoryForSkill(skillId: string): ConsentCategory | undefined {
  const tier = getSkillTier(skillId);
  if (tier === 'email' || tier === 'money' || tier === 'health') {
    return tier;
  }
  return undefined;
}

export function isKnownTierSkill(skillId: string): boolean {
  return skillId in SKILL_TIER_REGISTRY;
}
