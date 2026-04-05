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

export function normalizeReasoningInput<T extends IntentClassificationInput | ProcessRequest>(input: T): NormalizedReasoningRequest {
  const userProfile: UserProfile = {
    ...(input.user_profile ?? {})
  };

  const preferences = mergePreferences(userProfile, input.user_preferences);

  return {
    ...input,
    deployment_mode: input.deployment_mode ?? 'cloud',
    user_tier: input.user_tier ?? 'free',
    channel: input.channel,
    user_profile: {
      ...userProfile,
      enabled_skills: dedupeStrings(userProfile.enabled_skills),
      recent_intents: dedupeStrings(userProfile.recent_intents),
      preferences
    },
    user_preferences: preferences
  };
}
