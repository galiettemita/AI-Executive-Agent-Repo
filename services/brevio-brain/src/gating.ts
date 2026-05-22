import { randomUUID } from 'node:crypto';

import { getCategoryForSkill, getToolDescriptor } from './catalog.js';
import type {
  BundleSuggestion,
  ConsentRequirementResponse,
  CredentialRequirementResponse,
  NormalizedReasoningRequest,
  PlannerProposal
} from './types.js';

export interface GatingResult {
  outcome: 'pass' | 'requires_consent' | 'requires_credentials';
  requires_consent?: ConsentRequirementResponse;
  requires_credentials_for?: CredentialRequirementResponse;
  bundle_suggestion?: BundleSuggestion;
}

const CATEGORY_PROMPT_COPY: Record<'email' | 'money' | 'health', string> = {
  email: 'This needs your okay to read and send email — turn on?',
  money: 'This needs your okay to read your finance data — turn on?',
  health: 'This needs your okay to read your health data — turn on?'
};

const PROVIDER_EXPLANATION_COPY: Record<string, (skill: string) => string> = {
  google: (skill) =>
    `Brevio will connect to Google so it can ${prettyForSkill(skill)}. We don't share this with anyone.`,
  microsoft: (skill) =>
    `Brevio will connect to your Microsoft account so it can ${prettyForSkill(skill)}. We don't share this with anyone.`,
  apple: (skill) =>
    `Brevio will connect to your Apple account so it can ${prettyForSkill(skill)}.`,
  spotify: (skill) =>
    `Brevio will connect to Spotify so it can ${prettyForSkill(skill)}.`,
  github: (skill) =>
    `Brevio will connect to GitHub so it can ${prettyForSkill(skill)}.`,
  notion: (skill) =>
    `Brevio will connect to Notion so it can ${prettyForSkill(skill)}.`
};

function prettyForSkill(skillId: string): string {
  switch (skillId) {
    case 'google-calendar':
      return 'find times and add events you ask for';
    case 'google-workspace':
      return 'read your inbox and send messages on your behalf';
    case 'gkeep':
      return 'capture notes you ask it to';
    case 'outlook':
      return 'read your inbox and send messages on your behalf';
    case 'better-notion':
      return 'create and update pages on your behalf';
    case 'spotify':
    case 'spotify-web-api':
    case 'spotify-player':
      return 'control playback on your account';
    case 'spotify-history':
      return 'read your listening history to answer questions about it';
    default:
      return `do what you asked using ${skillId}`;
  }
}

function buildConsentRequirement(category: 'email' | 'money' | 'health', skillId: string): ConsentRequirementResponse {
  const pendingId = randomUUID();
  return {
    category,
    skill_id: skillId,
    prompt_copy: CATEGORY_PROMPT_COPY[category],
    accept_action: {
      method: 'POST',
      path: '/api/v1/me/consent',
      body: { category, state: 'granted', source: 'inline_prompt' }
    },
    decline_action: {
      method: 'POST',
      path: '/api/v1/me/consent',
      body: { category, state: 'snoozed', source: 'inline_prompt' }
    },
    pending_message_id: pendingId
  };
}

function buildCredentialRequirement(skillId: string, provider: string, scopes: string[]): CredentialRequirementResponse {
  const pendingId = randomUUID();
  const explanation = PROVIDER_EXPLANATION_COPY[provider]?.(skillId)
    ?? `Brevio needs to connect to ${provider} to do this. We don't share this with anyone.`;
  const params = new URLSearchParams({
    provider,
    skill_id: skillId,
    pending_message_id: pendingId
  });
  return {
    skill_id: skillId,
    provider,
    scopes,
    explanation_copy: explanation,
    connect_url: `/api/v1/oauth/start?${params.toString()}`,
    pending_message_id: pendingId
  };
}

export function evaluateGating(plan: PlannerProposal, request: NormalizedReasoningRequest): GatingResult {
  const grantedCategories = new Set(request.user_profile.granted_categories ?? []);
  const connectedProviders = new Set(request.user_profile.connected_providers ?? []);

  for (const action of plan.actions) {
    if (action.action_type !== 'execute_skill' || !action.skill_id) {
      continue;
    }
    const descriptor = getToolDescriptor(action.skill_id);
    if (!descriptor) {
      continue;
    }

    const category = getCategoryForSkill(action.skill_id);
    if (category && !grantedCategories.has(category)) {
      return {
        outcome: 'requires_consent',
        requires_consent: buildConsentRequirement(category, action.skill_id)
      };
    }

    const provider = descriptor.oauth_provider;
    if (provider && !connectedProviders.has(provider)) {
      return {
        outcome: 'requires_credentials',
        requires_credentials_for: buildCredentialRequirement(action.skill_id, provider, descriptor.oauth_scopes ?? [])
      };
    }
  }

  return { outcome: 'pass' };
}

export function evaluateBundleSuggestion(
  plan: PlannerProposal,
  request: NormalizedReasoningRequest
): BundleSuggestion | undefined {
  const connected = new Set(request.user_profile.connected_providers ?? []);
  const granted = new Set(request.user_profile.granted_categories ?? []);

  for (const action of plan.actions) {
    if (!action.skill_id) continue;
    const desc = getToolDescriptor(action.skill_id);
    if (!desc?.oauth_provider) continue;

    if (!connected.has(desc.oauth_provider)) continue;

    const category = getCategoryForSkill(action.skill_id);
    if (category && !granted.has(category)) {
      const params = new URLSearchParams({
        provider: desc.oauth_provider,
        skill_id: action.skill_id
      });
      return {
        provider: desc.oauth_provider,
        skill_id: action.skill_id,
        copy: `You're already connected to ${desc.oauth_provider}. Want to enable ${category} too? Same account, one tap.`,
        accept_url: `/api/v1/oauth/start?${params.toString()}`
      };
    }
  }

  return undefined;
}
