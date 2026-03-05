import type { DisambiguationRequest, DisambiguationResponse, DisambiguationRule, UserPreferences } from './types.js';

function ensureUnique(skills: string[]): string[] {
  return [...new Set(skills.filter((entry) => entry.trim() !== ''))];
}

function includesAny(text: string, patterns: string[]): boolean {
  const normalized = text.toLowerCase();
  return patterns.some((pattern) => normalized.includes(pattern));
}

function selectEmailProvider(preferences: UserPreferences | undefined): string {
  const provider = preferences?.email_provider ?? 'none';
  switch (provider) {
    case 'google':
      return 'google-workspace';
    case 'microsoft':
      return 'outlook';
    case 'apple':
      return 'apple-mail';
    case 'imap':
      return 'imap-email';
    default:
      return 'smtp-send';
  }
}

function selectSpotifySkill(mode: string | undefined, message: string): string {
  if (includesAny(message, ['history', 'top tracks', 'analytics'])) {
    return 'spotify-history';
  }

  if (mode === 'local_mac') {
    return 'spotify';
  }

  if (includesAny(message, ['terminal', 'cli'])) {
    return 'spotify-player';
  }

  return 'spotify-web-api';
}

function selectFlightSkill(message: string, userTier: string): string {
  const normalized = message.toLowerCase();
  if (normalized.includes('track')) {
    return userTier === 'free' ? 'flight-tracker' : 'aviationstack-flight-tracker';
  }
  if (normalized.includes('find')) {
    return 'aerobase-skill';
  }
  return userTier === 'free' ? 'flight-tracker' : 'aviationstack-flight-tracker';
}

function selectPackageSkill(message: string): string {
  const upper = message.toUpperCase();
  if (upper.includes('POST.AT') || upper.includes('AUSTRIA') || /\bAT\d{9,}\b/.test(upper)) {
    return 'post-at';
  }
  if (/^[A-Z0-9]{8,30}$/.test(upper.replace(/\s+/g, ''))) {
    return 'track17';
  }
  return 'parcel-package-tracking';
}

function selectPlacesSkill(message: string): string {
  const normalized = message.toLowerCase();
  if (normalized.includes('navigate to') || normalized.includes('directions')) {
    return 'google-maps';
  }
  if (normalized.includes('find all') || normalized.includes('every')) {
    return 'spots';
  }
  if (normalized.includes('near me')) {
    return 'goplaces';
  }
  return 'local-places';
}

function selectYouTubeSkill(message: string): string {
  const normalized = message.toLowerCase();
  if (normalized.includes('summarize')) {
    return 'youtube-summarizer';
  }
  if (normalized.includes('download transcript') || normalized.includes('full transcript')) {
    return 'video-transcript-downloader';
  }
  return 'youtube-api';
}

function applyRule(
  request: DisambiguationRequest,
  candidateSkills: string[],
  resolved: Set<string>,
  groupHits: string[]
): void {
  const message = request.message_text.toLowerCase();
  const userTier = request.user_tier ?? 'free';

  if (candidateSkills.includes('apple-notes') || candidateSkills.includes('apple-notes-skill')) {
    resolved.add('apple-notes-skill');
    groupHits.push('apple-notes');
  }

  if (candidateSkills.includes('notion') || candidateSkills.includes('better-notion')) {
    resolved.add('better-notion');
    groupHits.push('notion');
  }

  if (candidateSkills.some((skill) => skill.startsWith('spotify')) || message.includes('spotify')) {
    resolved.add(selectSpotifySkill(request.deployment_mode, message));
    groupHits.push('spotify');
  }

  if (candidateSkills.includes('aviationstack-flight-tracker') || candidateSkills.includes('flight-tracker') || message.includes('flight')) {
    resolved.add(selectFlightSkill(message, userTier));
    groupHits.push('flight-tracking');
  }

  if (candidateSkills.includes('healthkit-sync') || candidateSkills.includes('healthkit-sync-apple')) {
    resolved.add('healthkit-sync-apple');
    groupHits.push('healthkit');
  }

  if (candidateSkills.includes('apple-mail') || candidateSkills.includes('apple-mail-search') || message.includes('email')) {
    resolved.add(includesAny(message, ['search email', 'find email', 'search inbox']) ? 'apple-mail-search' : 'apple-mail');
    groupHits.push('apple-mail');
  }

  if (message.includes('send email') || message.includes('email ')) {
    resolved.add(selectEmailProvider(request.user_preferences));
    groupHits.push('email-send');
  }

  if (candidateSkills.includes('smart-expense-tracker') || candidateSkills.includes('expense-tracker-pro') || message.includes('expense')) {
    resolved.add('smart-expense-tracker');
    groupHits.push('expense-tracking');
  }

  if (candidateSkills.includes('parcel-package-tracking') || candidateSkills.includes('track17') || message.includes('package')) {
    resolved.add(selectPackageSkill(message));
    groupHits.push('package-tracking');
  }

  if (candidateSkills.includes('google-maps') || candidateSkills.includes('goplaces') || message.includes('near me') || message.includes('navigate')) {
    resolved.add(selectPlacesSkill(message));
    groupHits.push('places-location');
  }

  if (candidateSkills.includes('youtube-api') || candidateSkills.includes('youtube-summarizer') || message.includes('youtube')) {
    resolved.add(selectYouTubeSkill(message));
    groupHits.push('youtube');
  }
}

export function disambiguateSkills(request: DisambiguationRequest, rules: DisambiguationRule[]): DisambiguationResponse {
  const candidateSkills = ensureUnique(request.candidate_skills ?? []);
  const resolved = new Set<string>();
  const groupHits: string[] = [];

  applyRule(request, candidateSkills, resolved, groupHits);

  if (resolved.size === 0 && candidateSkills.length > 0) {
    for (const skill of candidateSkills) {
      resolved.add(skill);
    }
  }

  if (resolved.size === 0) {
    const fallback = request.intent?.includes('task') ? 'doing-tasks' : 'thinking-partner';
    resolved.add(fallback);
    groupHits.push('fallback');
  }

  const availableGroups = new Set(rules.map((rule) => rule.group));
  const uniqueGroupHits = ensureUnique(groupHits).filter((group) => availableGroups.has(group) || group === 'fallback');

  return {
    resolved_skills: ensureUnique(Array.from(resolved)),
    group_hits: uniqueGroupHits
  };
}
