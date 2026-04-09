import { createHash, randomUUID } from 'node:crypto';

import { parseCapabilityInventory, resolveCapabilityInventory } from '../../../packages/shared/src/capability-inventory.js';

import type { Channel, ContentType, GatewayConfig, MessageEnvelope, NormalizedWebhookResult, UserTier } from './types.js';
import { GatewayState } from './state.js';

interface GenericWebhookPayload {
  [key: string]: unknown;
}

function asObject(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

function asString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function asStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const out = value.map((entry) => asString(entry)).filter((entry): entry is string => Boolean(entry));
  return out.length > 0 ? out : undefined;
}

function asPositiveInt(value: unknown, max: number): number | undefined {
  if (typeof value !== 'number' || !Number.isInteger(value)) {
    return undefined;
  }
  if (value < 0 || value > max) {
    return undefined;
  }
  return value;
}

function contentTypeFromPayload(payload: GenericWebhookPayload): ContentType {
  const directType = asString(payload.type);
  const content = asObject(payload.content);
  const contentType = asString(content.type);
  const normalized = (contentType ?? directType ?? 'TEXT').toUpperCase();
  switch (normalized) {
    case 'VOICE':
    case 'IMAGE':
    case 'DOCUMENT':
    case 'LOCATION':
      return normalized;
    default:
      return 'TEXT';
  }
}

function timestampFromPayload(payload: GenericWebhookPayload, nowMs: number): string {
  const timestampRaw = asString(payload.timestamp);
  if (!timestampRaw) {
    return new Date(nowMs).toISOString();
  }
  const parsedMs = Date.parse(timestampRaw);
  if (Number.isNaN(parsedMs)) {
    return new Date(nowMs).toISOString();
  }
  return new Date(parsedMs).toISOString();
}

function userIdFromPayload(payload: GenericWebhookPayload, channel: Channel): string {
  const explicit = asString(payload.user_id);
  if (explicit) {
    return explicit;
  }

  const sender = asString(payload.sender_id) ?? asString(payload.from) ?? randomUUID();
  const stable = createHash('sha256').update(`${channel}:${sender}`).digest('hex').slice(0, 32);
  return `${stable.slice(0, 8)}-${stable.slice(8, 12)}-4${stable.slice(13, 16)}-a${stable.slice(17, 20)}-${stable.slice(20)}`;
}

function dedupKeyFromPayload(payload: GenericWebhookPayload, channel: Channel, rawBody: Buffer): string {
  const metadata = asObject(payload.metadata);
  const explicit =
    asString(metadata.channel_message_id) ??
    asString(payload.channel_message_id) ??
    asString(payload.message_id) ??
    asString(payload.id);
  if (explicit) {
    return `${channel}:${explicit}`;
  }
  const fallback = createHash('sha256').update(rawBody).digest('hex').slice(0, 48);
  return `${channel}:hash:${fallback}`;
}

function pickText(payload: GenericWebhookPayload): string | undefined {
  const content = asObject(payload.content);
  return asString(content.text) ?? asString(payload.text) ?? asString(payload.transcript) ?? asString(payload.voice_transcript);
}

function pickMediaURL(payload: GenericWebhookPayload): string | undefined {
  const content = asObject(payload.content);
  return asString(content.media_url) ?? asString(payload.media_url) ?? asString(payload.attachment_url);
}

function profileHash(channel: Channel, userId: string): string {
  return createHash('sha256').update(`${channel}:${userId}`).digest('hex');
}

function tierFromPayload(payload: GenericWebhookPayload): UserTier | undefined {
  const raw = asString(payload.user_tier)?.toLowerCase();
  switch (raw) {
    case 'free':
    case 'pro':
    case 'enterprise':
    case 'admin':
    case 'service':
      return raw;
    default:
      return undefined;
  }
}

export function normalizeWebhook(
  channel: Channel,
  payloadRaw: unknown,
  rawBody: Buffer,
  nowMs: number,
  state: GatewayState,
  config: GatewayConfig,
  fallbackTier: UserTier
): NormalizedWebhookResult & { tier: UserTier } {
  const payload = asObject(payloadRaw);
  const metadata = asObject(payload.metadata);
  const context = asObject(payload.context);

  const dedupKey = dedupKeyFromPayload(payload, channel, rawBody);
  const dedupChannelMessageId = dedupKey.split(':').slice(1).join(':');
  const userId = userIdFromPayload(payload, channel);
  const sessionId = state.sessionForUser(
    channel,
    userId,
    asString(metadata.session_id) ?? asString(payload.session_id),
    nowMs,
    config.sessionIdleMs
  );

  const contentType = contentTypeFromPayload(payload);
  const text = pickText(payload);
  const mediaURL = pickMediaURL(payload);
  const voiceDurationMs = asPositiveInt(asObject(payload.content).voice_duration_ms ?? payload.voice_duration_ms, 120000);
  const explicitActiveSkills = asStringArray(context.active_skills) ?? asStringArray(payload.active_skills);
  const tenantId = asString(context.tenant_id) ?? asString(metadata.tenant_id) ?? asString(payload.tenant_id);
  const workspaceId = asString(context.workspace_id) ?? asString(metadata.workspace_id) ?? asString(payload.workspace_id);
  const capabilityResolution = resolveCapabilityInventory(
    parseCapabilityInventory(config.capabilityInventoryJson),
    { tenantId, workspaceId, userId },
    explicitActiveSkills
  );
  const activeSkills = capabilityResolution.enabledSkills.length > 0 ? capabilityResolution.enabledSkills : explicitActiveSkills;

  const envelope: MessageEnvelope = {
    id: randomUUID(),
    channel,
    user_id: userId,
    timestamp: timestampFromPayload(payload, nowMs),
    content: {
      type: contentType,
      text: text,
      media_url: mediaURL,
      voice_duration_ms: voiceDurationMs
    },
    metadata: {
      channel_message_id: dedupChannelMessageId,
      reply_to: asString(metadata.reply_to) ?? asString(payload.reply_to),
      session_id: sessionId
    },
    context: {
      user_profile_hash: profileHash(channel, userId),
      active_skills: activeSkills,
      capability_source: capabilityResolution.source,
      denied_skills: capabilityResolution.deniedSkills.length > 0 ? capabilityResolution.deniedSkills : undefined,
      tenant_id: tenantId,
      workspace_id: workspaceId
    }
  };

  if (envelope.content.type === 'VOICE' && !envelope.content.text) {
    envelope.content.text = '[voice message transcription unavailable]';
  }

  return {
    envelope,
    userId,
    dedupKey,
    tier: tierFromPayload(payload) ?? fallbackTier
  };
}
