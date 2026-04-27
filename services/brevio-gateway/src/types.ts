import type { AccessTokenIssuerRegistry, CallerContextIssuerRegistry } from '../../../packages/shared/src/security.js';

export type Channel = 'WHATSAPP' | 'IMESSAGE' | 'API';

export type ContentType = 'TEXT' | 'VOICE' | 'IMAGE' | 'DOCUMENT' | 'LOCATION';

export type UserTier = 'free' | 'pro' | 'enterprise' | 'admin' | 'service';

export interface MessageEnvelope {
  id: string;
  channel: Channel;
  user_id: string;
  timestamp: string;
  content: {
    type: ContentType;
    text?: string;
    media_url?: string;
    voice_duration_ms?: number;
  };
  metadata: {
    channel_message_id: string;
    reply_to?: string;
    session_id: string;
  };
  context: {
    user_profile_hash: string;
    active_skills?: string[];
    capability_source?: 'explicit' | 'inventory' | 'merged' | 'none';
    denied_skills?: string[];
    tenant_id?: string;
    workspace_id?: string;
  };
}

export interface GatewayConfig {
  serviceName: string;
  version: string;
  environment: string;
  port: number;
  shutdownTimeoutMs: number;
  stateFilePath?: string;
  accessTokenIssuers: AccessTokenIssuerRegistry;
  adminTokenIssuer: string;
  serviceTokenSigningKey: string;
  serviceTokenIssuer: string;
  serviceAudience: string;
  temporalWorkerAudience: string;
  callerContextIssuers: CallerContextIssuerRegistry;
  callerContextSigningKey: string;
  callerContextIssuer: string;
  logSalt: string;

  whatsappWebhookSecret: string;
  whatsappVerifyToken: string;
  imessageAPIKey: string;
  temporalWebhookAPIKey: string;
  temporalWorkerBaseUrl?: string;
  temporalWorkerTimeoutMs: number;
  capabilityInventoryJson?: string;

  idempotencyTtlMs: number;
  sessionIdleMs: number;

  rateLimitWindowMs: number;
  rateLimitMinuteWindowMs: number;
  rateLimitPerMinute: number;
  rateLimitFreePerHour: number;
  rateLimitProPerHour: number;
}

export interface RequestContext {
  traceId: string;
  spanId: string;
  requestId: string;
  subjectRef?: string;
}

export interface RateLimitDecision {
  allowed: boolean;
  reason?: 'RATE_LIMIT_MINUTE' | 'RATE_LIMIT_HOUR';
  limit: number;
  remaining: number;
  retryAfterSeconds: number;
}

export interface DedupCachedResponse {
  statusCode: number;
  payload: Record<string, unknown>;
  expiresAtMs: number;
}

export interface SessionState {
  sessionId: string;
  lastActivityMs: number;
}

export interface NormalizedWebhookResult {
  envelope: MessageEnvelope;
  userId: string;
  dedupKey: string;
  channelSubject: string;
}
