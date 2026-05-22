import type { DeploymentMode, UserPreferences } from './types.js';

export type SafetyTier = 'safe' | 'email' | 'money' | 'health' | 'dangerous';
export type OAuthProvider = 'google' | 'microsoft' | 'apple' | 'spotify' | 'github' | 'notion';

export interface ToolDescriptor {
  skill_id: string;
  group?: string;
  aliases: string[];
  operations: string[];
  write_operations: string[];
  deployment_modes: DeploymentMode[];
  requires_confirmation?: boolean;
  safety_tier: SafetyTier;
  oauth_provider?: OAuthProvider;
  oauth_scopes?: string[];
}

const TOOL_CATALOG_ENTRIES: ToolDescriptor[] = [
  { skill_id: 'aerobase-skill', group: 'flight-tracking', aliases: ['flight search'], operations: ['find'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'apple-mail', group: 'apple-mail', aliases: ['mail'], operations: ['list_inbox', 'search', 'send', 'reply'], write_operations: ['send', 'reply'], deployment_modes: ['local_mac'], safety_tier: 'email' },
  { skill_id: 'apple-mail-search', group: 'apple-mail', aliases: ['mail search'], operations: ['search_all', 'search_sender', 'search_subject'], write_operations: [], deployment_modes: ['local_mac'], safety_tier: 'email' },
  { skill_id: 'apple-music', aliases: ['music'], operations: ['play'], write_operations: [], deployment_modes: ['local_mac'], safety_tier: 'safe' },
  { skill_id: 'apple-notes-skill', group: 'apple-notes', aliases: ['notes'], operations: ['create_note'], write_operations: ['create_note'], deployment_modes: ['local_mac', 'cloud'], safety_tier: 'safe' },
  { skill_id: 'asana', aliases: ['tasks'], operations: ['create'], write_operations: ['create'], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'aviationstack-flight-tracker', group: 'flight-tracking', aliases: ['flight status'], operations: ['track'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'bear-notes', aliases: ['notes'], operations: ['create_note'], write_operations: ['create_note'], deployment_modes: ['local_mac'], safety_tier: 'safe' },
  { skill_id: 'better-notion', group: 'notion', aliases: ['notion'], operations: ['create_page'], write_operations: ['create_page'], deployment_modes: ['cloud'], safety_tier: 'safe', oauth_provider: 'notion', oauth_scopes: ['read_content', 'update_content'] },
  { skill_id: 'brave-search', aliases: ['research'], operations: ['search'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'clickup-mcp', aliases: ['tasks'], operations: ['create'], write_operations: ['create'], deployment_modes: ['mcp'], safety_tier: 'safe' },
  { skill_id: 'craft', aliases: ['notes'], operations: ['create_note'], write_operations: ['create_note'], deployment_modes: ['cloud', 'local_mac'], safety_tier: 'safe' },
  { skill_id: 'firecrawl-search', aliases: ['research'], operations: ['search'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'flight-tracker', group: 'flight-tracking', aliases: ['flight status'], operations: ['track'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'google-calendar', aliases: ['calendar'], operations: ['list', 'create', 'update', 'delete'], write_operations: ['create', 'update', 'delete'], deployment_modes: ['cloud', 'mcp'], safety_tier: 'safe', oauth_provider: 'google', oauth_scopes: ['https://www.googleapis.com/auth/calendar.events'] },
  { skill_id: 'google-maps', group: 'places-location', aliases: ['maps', 'directions'], operations: ['navigate'], write_operations: [], deployment_modes: ['cloud', 'local_mac'], safety_tier: 'safe' },
  { skill_id: 'google-workspace', group: 'email-send', aliases: ['gmail', 'google'], operations: ['gmail_list', 'gmail_send', 'calendar_list', 'drive_search'], write_operations: ['gmail_send'], deployment_modes: ['cloud'], safety_tier: 'email', oauth_provider: 'google', oauth_scopes: ['https://www.googleapis.com/auth/gmail.modify', 'https://www.googleapis.com/auth/gmail.send'] },
  { skill_id: 'goplaces', group: 'places-location', aliases: ['near me'], operations: ['search'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'gkeep', aliases: ['notes'], operations: ['create_note'], write_operations: ['create_note'], deployment_modes: ['cloud'], safety_tier: 'safe', oauth_provider: 'google', oauth_scopes: ['https://www.googleapis.com/auth/keep'] },
  { skill_id: 'healthkit-sync-apple', group: 'healthkit', aliases: ['healthkit'], operations: ['sync'], write_operations: [], deployment_modes: ['local_mac'], safety_tier: 'health' },
  { skill_id: 'imap-email', aliases: ['email'], operations: ['list', 'search', 'send'], write_operations: ['send'], deployment_modes: ['cloud', 'local_mac'], safety_tier: 'email' },
  { skill_id: 'jira', aliases: ['tasks'], operations: ['create'], write_operations: ['create'], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'linear', aliases: ['tasks'], operations: ['create'], write_operations: ['create'], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'local-places', group: 'places-location', aliases: ['nearby'], operations: ['search'], write_operations: [], deployment_modes: ['local_mac', 'cloud'], safety_tier: 'safe' },
  { skill_id: 'obsidian', aliases: ['notes'], operations: ['create_note'], write_operations: ['create_note'], deployment_modes: ['local_mac'], safety_tier: 'safe' },
  { skill_id: 'omnifocus', aliases: ['tasks'], operations: ['create'], write_operations: ['create'], deployment_modes: ['local_mac'], safety_tier: 'safe' },
  { skill_id: 'outlook', group: 'email-send', aliases: ['outlook'], operations: ['inbox_list', 'send', 'calendar_list'], write_operations: ['send'], deployment_modes: ['cloud'], safety_tier: 'email', oauth_provider: 'microsoft', oauth_scopes: ['Mail.Read', 'Mail.Send', 'Calendars.Read'] },
  { skill_id: 'parcel-package-tracking', group: 'package-tracking', aliases: ['package tracking'], operations: ['track'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'post-at', group: 'package-tracking', aliases: ['austrian post'], operations: ['track'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'reflect', aliases: ['notes'], operations: ['create_note'], write_operations: ['create_note'], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'shopping-expert', aliases: ['shopping'], operations: ['research'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'smart-expense-tracker', group: 'expense-tracking', aliases: ['expenses'], operations: ['analyze'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'money' },
  { skill_id: 'smtp-send', aliases: ['email'], operations: ['send'], write_operations: ['send'], deployment_modes: ['cloud'], safety_tier: 'email' },
  { skill_id: 'spotify', group: 'spotify', aliases: ['spotify'], operations: ['play'], write_operations: [], deployment_modes: ['local_mac'], safety_tier: 'safe', oauth_provider: 'spotify', oauth_scopes: ['user-modify-playback-state', 'user-read-playback-state'] },
  { skill_id: 'spotify-history', group: 'spotify', aliases: ['spotify stats'], operations: ['analytics'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe', oauth_provider: 'spotify', oauth_scopes: ['user-read-recently-played', 'user-top-read'] },
  { skill_id: 'spotify-player', group: 'spotify', aliases: ['spotify cli'], operations: ['play'], write_operations: [], deployment_modes: ['terminal'], safety_tier: 'safe', oauth_provider: 'spotify', oauth_scopes: ['user-modify-playback-state'] },
  { skill_id: 'spotify-web-api', group: 'spotify', aliases: ['spotify'], operations: ['play'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe', oauth_provider: 'spotify', oauth_scopes: ['user-modify-playback-state', 'user-read-playback-state'] },
  { skill_id: 'spots', group: 'places-location', aliases: ['all nearby'], operations: ['search_all'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'tavily', aliases: ['research'], operations: ['search'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'things-mac', aliases: ['tasks'], operations: ['create'], write_operations: ['create'], deployment_modes: ['local_mac'], safety_tier: 'safe' },
  { skill_id: 'thinking-partner', aliases: ['reasoning'], operations: ['clarify'], write_operations: [], deployment_modes: ['cloud', 'local_mac', 'mcp', 'terminal'], safety_tier: 'safe' },
  { skill_id: 'ticktick', aliases: ['tasks'], operations: ['create'], write_operations: ['create'], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'todoist', aliases: ['tasks'], operations: ['list', 'create', 'complete', 'delete'], write_operations: ['create', 'complete', 'delete'], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'track17', group: 'package-tracking', aliases: ['17track'], operations: ['track'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'trello', aliases: ['tasks'], operations: ['create'], write_operations: ['create'], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'watch-my-money', aliases: ['expenses'], operations: ['analyze'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'money' },
  { skill_id: 'ynab', aliases: ['expenses'], operations: ['analyze'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'money' },
  { skill_id: 'youtube-api', group: 'youtube', aliases: ['youtube'], operations: ['search'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'youtube-summarizer', group: 'youtube', aliases: ['youtube summary'], operations: ['summarize'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'video-transcript-downloader', group: 'youtube', aliases: ['youtube transcript'], operations: ['download_transcript'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' },
  { skill_id: 'ytmusic', aliases: ['music'], operations: ['play'], write_operations: [], deployment_modes: ['cloud'], safety_tier: 'safe' }
];

export const TOOL_CATALOG: Record<string, ToolDescriptor> = Object.fromEntries(
  TOOL_CATALOG_ENTRIES.map((entry) => [entry.skill_id, entry])
);

const TASK_APP_TO_SKILL: Record<string, string> = {
  todoist: 'todoist',
  things: 'things-mac',
  ticktick: 'ticktick',
  omnifocus: 'omnifocus',
  trello: 'trello',
  asana: 'asana',
  linear: 'linear',
  jira: 'jira',
  clickup: 'clickup-mcp',
  apple_reminders: 'apple-remind-me'
};

const NOTES_APP_TO_SKILL: Record<string, string> = {
  apple_notes: 'apple-notes-skill',
  notion: 'better-notion',
  bear: 'bear-notes',
  obsidian: 'obsidian',
  craft: 'craft',
  google_keep: 'gkeep',
  reflect: 'reflect'
};

const CATEGORY_PROVIDERS: Record<'email' | 'money' | 'health', OAuthProvider[]> = {
  email: ['google', 'microsoft'],
  money: [],
  health: []
};

export function getToolDescriptor(skillId: string | undefined): ToolDescriptor | undefined {
  if (!skillId) {
    return undefined;
  }
  return TOOL_CATALOG[skillId];
}

export function toolExists(skillId: string | undefined): boolean {
  return Boolean(getToolDescriptor(skillId));
}

export function resolveTaskSkill(preferences: UserPreferences | undefined): string {
  const taskApp = preferences?.task_app ?? 'todoist';
  return TASK_APP_TO_SKILL[taskApp] ?? 'todoist';
}

export function resolveNotesSkill(preferences: UserPreferences | undefined): string {
  const notesApp = preferences?.notes_app ?? 'apple_notes';
  return NOTES_APP_TO_SKILL[notesApp] ?? 'apple-notes-skill';
}

export function resolveMusicSkill(preferences: UserPreferences | undefined, deploymentMode: DeploymentMode | undefined): string | undefined {
  const preferred = preferences?.music_provider ?? 'none';
  if (preferred === 'none') {
    return undefined;
  }
  if (preferred === 'apple_music') {
    return 'apple-music';
  }
  if (preferred === 'youtube_music') {
    return 'ytmusic';
  }
  if (deploymentMode === 'local_mac') {
    return 'spotify';
  }
  if (deploymentMode === 'terminal') {
    return 'spotify-player';
  }
  return 'spotify-web-api';
}

export function resolveEmailSkill(
  preferences: UserPreferences | undefined,
  operation: 'send' | 'search',
  deploymentMode: DeploymentMode | undefined
): string | undefined {
  const provider = preferences?.email_provider ?? 'none';
  switch (provider) {
    case 'google':
      return 'google-workspace';
    case 'microsoft':
      return 'outlook';
    case 'apple':
      return operation === 'search' ? 'apple-mail-search' : 'apple-mail';
    case 'imap':
      return 'imap-email';
    case 'none':
    default:
      if (operation === 'search' && deploymentMode === 'local_mac') {
        return undefined;
      }
      return undefined;
  }
}

export function findGroupForSkill(skillId: string | undefined): string | undefined {
  return getToolDescriptor(skillId)?.group;
}

export function filterEnabledSkills(skills: string[], enabledSkills: string[] | undefined): { allowed: string[]; blocked: string[] } {
  const unique = [...new Set(skills.filter((skill) => toolExists(skill)))];
  if (!enabledSkills || enabledSkills.length === 0) {
    return { allowed: [], blocked: unique };
  }
  const enabled = new Set(enabledSkills);
  return {
    allowed: unique.filter((skill) => enabled.has(skill)),
    blocked: unique.filter((skill) => !enabled.has(skill))
  };
}

export function getSkillsByTier(tier: SafetyTier): string[] {
  return TOOL_CATALOG_ENTRIES
    .filter((entry) => entry.safety_tier === tier)
    .map((entry) => entry.skill_id);
}

export function getDefaultEnabledSkills(): string[] {
  return getSkillsByTier('safe');
}

export function getSkillsForProvider(provider: OAuthProvider): string[] {
  return TOOL_CATALOG_ENTRIES
    .filter((entry) => entry.oauth_provider === provider)
    .map((entry) => entry.skill_id);
}

export function getOAuthScopesForSkill(skillId: string): string[] {
  return getToolDescriptor(skillId)?.oauth_scopes ?? [];
}

export function getProvidersForCategory(category: 'email' | 'money' | 'health'): OAuthProvider[] {
  return [...CATEGORY_PROVIDERS[category]];
}

export function getCategoryForSkill(skillId: string): 'email' | 'money' | 'health' | undefined {
  const tier = getToolDescriptor(skillId)?.safety_tier;
  if (tier === 'email' || tier === 'money' || tier === 'health') {
    return tier;
  }
  return undefined;
}
