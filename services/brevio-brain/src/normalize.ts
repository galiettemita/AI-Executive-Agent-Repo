import { getDefaultEnabledSkills, getSkillsByTier } from './catalog.js';
import type {
  IntentClassificationInput,
  NormalizedReasoningRequest,
  ProcessRequest,
  UserPreferences,
  UserProfile
} from './types.js';

function dedupeStrings(values: string[] | undefined): string[] | undefined {
  if (!values || values.length === 0) {
    return undefined;
  }
  const unique = [...new Set(values.map((value) => value.trim()).filter((value) => value.length > 0))];
  return unique.length > 0 ? unique : undefined;
}

function mergePreferences(profile: UserProfile | undefined, topLevel: UserPreferences | undefined): UserPreferences {
  return {
    ...(profile?.preferences ?? {}),
    ...(topLevel ?? {})
  };
}

function computeDefaultEnabledSkills(grantedCategories: Array<'email' | 'money' | 'health'> | undefined): string[] {
  const skills = [...getDefaultEnabledSkills()];
  if (grantedCategories?.includes('email')) skills.push(...getSkillsByTier('email'));
  if (grantedCategories?.includes('money')) skills.push(...getSkillsByTier('money'));
  if (grantedCategories?.includes('health')) skills.push(...getSkillsByTier('health'));
  return [...new Set(skills)];
}

export function normalizeReasoningInput<T extends IntentClassificationInput | ProcessRequest>(input: T): NormalizedReasoningRequest {
  const userProfile: UserProfile = {
    ...(input.user_profile ?? {})
  };

  const preferences = mergePreferences(userProfile, input.user_preferences);

  const explicitEnabled = dedupeStrings(userProfile.enabled_skills);
  const grantedCategories = userProfile.granted_categories ?? [];
  const enabled_skills = explicitEnabled ?? computeDefaultEnabledSkills(grantedCategories);

  return {
    ...input,
    deployment_mode: input.deployment_mode ?? 'cloud',
    user_tier: input.user_tier ?? 'free',
    channel: input.channel,
    user_profile: {
      ...userProfile,
      enabled_skills,
      recent_intents: dedupeStrings(userProfile.recent_intents),
      granted_categories: grantedCategories,
      connected_providers: userProfile.connected_providers ?? [],
      preferences
    },
    user_preferences: preferences
  };
}
