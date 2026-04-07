import { filterEnabledSkills, findGroupForSkill, resolveEmailSkill, toolExists } from './catalog.js';
import type { DisambiguationRequest, DisambiguationResponse, DisambiguationRuleConfig, DisambiguationRules } from './types.js';

function unique(values: string[]): string[] {
  return [...new Set(values.filter((value) => value.trim().length > 0))];
}

function includesAny(text: string, patterns: string[]): boolean {
  return patterns.some((pattern) => text.includes(pattern));
}

function detectGroup(intent: string | undefined, candidateSkills: string[]): string | undefined {
  const intentToGroup: Record<string, string> = {
    'notes.capture': 'apple-notes',
    'transport.flight_tracking': 'flight-tracking',
    'transport.flight_search': 'flight-tracking',
    'finance.expense': 'expense-tracking',
    'video.youtube': 'youtube',
    'places.search': 'places-location',
    'email.send': 'email-send',
    'email.search': 'apple-mail'
  };

  if (intent && intentToGroup[intent]) {
    return intentToGroup[intent];
  }

  for (const skill of candidateSkills) {
    const group = findGroupForSkill(skill);
    if (group) {
      return group;
    }
  }

  return undefined;
}

function resolveFromRule(
  rule: DisambiguationRuleConfig,
  request: DisambiguationRequest,
  message: string
): { skills: string[]; reasoning: string[]; clarificationRequired: boolean } {
  const reasoning: string[] = [];
  const userTier = request.user_tier ?? 'free';
  const deploymentMode = request.deployment_mode ?? 'cloud';
  const emailPreference = request.user_preferences?.email_provider ?? 'none';

  switch (rule.group) {
    case 'apple-notes':
      reasoning.push('Using canonical Apple Notes route.');
      return { skills: [rule.canonical ?? 'apple-notes-skill'], reasoning, clarificationRequired: false };
    case 'notion':
      if (message.includes('legacy') && rule.fallback) {
        reasoning.push('Legacy Notion explicitly requested.');
        return { skills: [rule.fallback], reasoning, clarificationRequired: false };
      }
      reasoning.push('Using modern Notion route.');
      return { skills: [rule.canonical ?? 'better-notion'], reasoning, clarificationRequired: false };
    case 'spotify':
      if (includesAny(message, ['history', 'top artist', 'top track', 'analytics', 'stats']) && rule.analytics) {
        reasoning.push('Spotify analytics intent detected.');
        return { skills: [rule.analytics], reasoning, clarificationRequired: false };
      }
      if (deploymentMode === 'local_mac' && rule.local_mac) {
        reasoning.push('Local mac deployment selected Spotify local adapter.');
        return { skills: [rule.local_mac], reasoning, clarificationRequired: false };
      }
      if (deploymentMode === 'terminal' && rule.terminal) {
        reasoning.push('Terminal deployment selected Spotify CLI adapter.');
        return { skills: [rule.terminal], reasoning, clarificationRequired: false };
      }
      reasoning.push('Cloud/default deployment selected Spotify web API adapter.');
      return { skills: [rule.cloud ?? 'spotify-web-api'], reasoning, clarificationRequired: false };
    case 'flight-tracking':
      if (request.intent === 'transport.flight_search' || includesAny(message, ['find flight', 'search flight', 'book flight', 'cheapest flight'])) {
        reasoning.push('Flight discovery intent resolved to search skill.');
        return { skills: [rule.find ?? 'aerobase-skill'], reasoning, clarificationRequired: false };
      }
      if (userTier === 'free') {
        reasoning.push('Free tier resolved to lightweight tracker.');
        return { skills: [rule.free_tier ?? 'flight-tracker'], reasoning, clarificationRequired: false };
      }
      reasoning.push('Paid tier resolved to full tracker.');
      return { skills: [rule.track ?? 'aviationstack-flight-tracker'], reasoning, clarificationRequired: false };
    case 'healthkit':
      reasoning.push('Using canonical HealthKit route.');
      return { skills: [rule.canonical ?? 'healthkit-sync-apple'], reasoning, clarificationRequired: false };
    case 'apple-mail': {
      const search = includesAny(message, ['search', 'find email', 'look up mail', 'search inbox', 'latest email']);
      const send = includesAny(message, ['send email', 'draft email', 'compose email', 'reply to email']);
      if (search && send && !request.allow_multi_intent) {
        reasoning.push('Email send and search cues were both present; clarification required.');
        return { skills: [], reasoning, clarificationRequired: true };
      }
      if (search) {
        reasoning.push('Email search intent routed to low-latency search skill.');
        return { skills: [rule.search ?? 'apple-mail-search'], reasoning, clarificationRequired: false };
      }
      reasoning.push('Defaulting to Apple Mail CRUD route.');
      return { skills: [rule.crud ?? 'apple-mail'], reasoning, clarificationRequired: false };
    }
    case 'email-send':
      if (emailPreference === 'none') {
        reasoning.push('Email execution requires an approved provider before routing can continue.');
        return {
          skills: [],
          reasoning,
          clarificationRequired: true
        };
      }
      reasoning.push('Email routing used the approved provider preference.');
      return {
        skills: [resolveEmailSkill(request.user_preferences, 'send', request.deployment_mode)].filter((skill): skill is string => Boolean(skill)),
        reasoning,
        clarificationRequired: false
      };
    case 'expense-tracking':
      reasoning.push('Expense orchestration uses the canonical tracker.');
      return { skills: [rule.canonical ?? 'smart-expense-tracker'], reasoning, clarificationRequired: false };
    case 'package-tracking':
      if (/^[A-Z]{2}[0-9]{9}AT$/.test(message.toUpperCase()) || includesAny(message, ['austrian post', 'post.at'])) {
        reasoning.push('Austrian Post format detected.');
        return { skills: [rule.austrian_post ?? 'post-at'], reasoning, clarificationRequired: false };
      }
      if (/^[A-Z]{2}[0-9]{9}[A-Z]{2}$/.test(message.toUpperCase().replace(/\s+/g, '')) || includesAny(message, ['17track', 'yunexpress', 'yanwen', 'cainiao'])) {
        reasoning.push('17track-compatible carrier detected.');
        return { skills: [rule.carriers_17track ?? 'track17'], reasoning, clarificationRequired: false };
      }
      reasoning.push('Using default international package tracking route.');
      return { skills: [rule.international ?? 'parcel-package-tracking'], reasoning, clarificationRequired: false };
    case 'places-location':
      if (includesAny(message, ['navigate', 'directions', 'route to', 'drive to', 'walk to'])) {
        reasoning.push('Navigation intent detected.');
        return { skills: [rule.navigate ?? 'google-maps'], reasoning, clarificationRequired: false };
      }
      if (includesAny(message, ['find all', 'all places', 'every place', 'all restaurants'])) {
        reasoning.push('Exhaustive places search detected.');
        return { skills: [rule.find_all ?? 'spots'], reasoning, clarificationRequired: false };
      }
      if (includesAny(message, ['near me', 'nearby', 'closest'])) {
        if (includesAny(message, ['quick', 'simple'])) {
          reasoning.push('Simple nearby search detected.');
          return { skills: [rule.simple_nearby ?? 'local-places'], reasoning, clarificationRequired: false };
        }
        reasoning.push('Rich nearby search detected.');
        return { skills: [rule.near_me ?? 'goplaces'], reasoning, clarificationRequired: false };
      }
      reasoning.push('Defaulting to nearby places route.');
      return { skills: [rule.simple_nearby ?? 'local-places'], reasoning, clarificationRequired: false };
    case 'youtube':
      if (includesAny(message, ['summarize', 'summary', 'tl;dr'])) {
        reasoning.push('YouTube summary intent detected.');
        return { skills: [rule.summarize ?? 'youtube-summarizer'], reasoning, clarificationRequired: false };
      }
      if (includesAny(message, ['download', 'transcript', 'subtitle', 'captions', 'full video'])) {
        reasoning.push('YouTube transcript/download intent detected.');
        return { skills: [rule.download ?? 'video-transcript-downloader'], reasoning, clarificationRequired: false };
      }
      reasoning.push('Defaulting to YouTube search route.');
      return { skills: [rule.search ?? 'youtube-api'], reasoning, clarificationRequired: false };
    default:
      return { skills: [], reasoning: [`No router available for group ${rule.group}.`], clarificationRequired: true };
  }
}

export function disambiguateSkills(request: DisambiguationRequest, rules: DisambiguationRules): DisambiguationResponse {
  const message = request.message_text.toLowerCase();
  const requestedSkills = unique((request.candidate_skills ?? []).filter((skill) => toolExists(skill)));
  const { allowed, blocked } = filterEnabledSkills(requestedSkills, request.enabled_skills);
  const reasoning: string[] = [];

  if (blocked.length > 0) {
    reasoning.push('One or more candidate skills are not approved for execution in the current capability set.');
  }

  const group = detectGroup(request.intent, allowed);
  if (!group) {
    if (allowed.length > 0) {
      return {
        resolved_skills: allowed,
        group_hits: [],
        blocked_skills: blocked,
        clarification_required: false,
        reasoning: [...reasoning, 'Planner kept the explicit approved connector selection.']
      };
    }
    const fallback = request.intent?.startsWith('tasks.') ? 'doing-tasks' : 'thinking-partner';
    const { allowed: allowedFallback, blocked: blockedFallback } = filterEnabledSkills([fallback], request.enabled_skills);
    return {
      resolved_skills: allowedFallback,
      group_hits: allowedFallback.length > 0 ? ['fallback'] : [],
      blocked_skills: [...blocked, ...blockedFallback],
      clarification_required: allowedFallback.length === 0,
      reasoning: [...reasoning, allowedFallback.length > 0 ? 'Fell back to a generic approved reasoning skill.' : 'No approved fallback skill is available.']
    };
  }

  const rule = rules[group];
  if (!rule) {
    return {
      resolved_skills: allowed,
      group_hits: [],
      blocked_skills: blocked,
      clarification_required: true,
      reasoning: [...reasoning, `No config rule found for disambiguation group ${group}.`]
    };
  }

  const routed = resolveFromRule(rule, request, message);
  const filtered = filterEnabledSkills(routed.skills, request.enabled_skills);

  return {
    resolved_skills: unique(filtered.allowed),
    group_hits: [group],
    blocked_skills: unique([...blocked, ...filtered.blocked]),
    clarification_required: routed.clarificationRequired || filtered.allowed.length === 0,
    reasoning: [...reasoning, ...routed.reasoning]
  };
}
