import path from 'node:path';

import {
  buildAccessTokenIssuerRegistry,
  buildCallerContextIssuerRegistry,
  loadBrevioEnvironment,
  resolveAccessTokenSigningKey,
  resolveAccessTokenVerificationKey,
  resolveCallerContextSigningKey,
  resolveCallerContextVerificationKey
} from '../../../packages/shared/src/security.js';

import type { GatewayConfig } from './types.js';

function parsePositiveInt(raw: string | undefined, fallback: number, field: string): number {
  if (!raw || raw.trim() === '') {
    return fallback;
  }
  const value = Number(raw);
  if (!Number.isInteger(value) || value <= 0) {
    throw new Error(`invalid ${field}: expected positive integer`);
  }
  return value;
}

export function loadGatewayConfig(): GatewayConfig {
  const environment = loadBrevioEnvironment();
  return {
    serviceName: 'brevio-gateway',
    version: process.env.SERVICE_VERSION ?? '0.2.0',
    environment,
    port: parsePositiveInt(process.env.PORT, 8080, 'PORT'),
    shutdownTimeoutMs: parsePositiveInt(process.env.BREVIO_GATEWAY_SHUTDOWN_TIMEOUT_MS, 30000, 'BREVIO_GATEWAY_SHUTDOWN_TIMEOUT_MS'),
    stateFilePath: path.resolve(process.env.BREVIO_GATEWAY_STATE_FILE ?? path.join(process.cwd(), 'data', 'gateway', 'state.json')),
    accessTokenIssuers: buildAccessTokenIssuerRegistry([
      {
        issuer: process.env.BREVIO_AUTH_ACCESS_ISSUER?.trim() || 'https://auth.brevio.internal',
        verificationKey: resolveAccessTokenVerificationKey(
          process.env.BREVIO_AUTH_ACCESS_PUBLIC_KEY,
          undefined,
          undefined,
          environment,
          'BREVIO_AUTH_ACCESS_PUBLIC_KEY',
          'auth-access'
        ),
        allowedTokenUses: ['user_access', 'admin_access']
      }
    ]),
    adminTokenIssuer: process.env.BREVIO_AUTH_ACCESS_ISSUER?.trim() || 'https://auth.brevio.internal',
    serviceTokenSigningKey: resolveAccessTokenSigningKey(
      process.env.BREVIO_GATEWAY_SERVICE_PRIVATE_KEY,
      undefined,
      environment,
      'BREVIO_GATEWAY_SERVICE_PRIVATE_KEY',
      'gateway-service'
    ),
    serviceTokenIssuer: process.env.BREVIO_GATEWAY_SERVICE_ISSUER?.trim() || 'https://gateway.brevio.internal',
    serviceAudience: process.env.BREVIO_GATEWAY_AUDIENCE?.trim() || 'brevio-gateway',
    temporalWorkerAudience: process.env.BREVIO_TEMPORAL_WORKER_AUDIENCE?.trim() || 'brevio-temporal-worker',
    callerContextIssuers: buildCallerContextIssuerRegistry([
      {
        issuer: process.env.BREVIO_GATEWAY_CALLER_CONTEXT_ISSUER?.trim() || 'https://gateway.brevio.internal/caller-context',
        verificationKey: resolveCallerContextVerificationKey(
          process.env.BREVIO_GATEWAY_CALLER_CONTEXT_PUBLIC_KEY,
          environment,
          'BREVIO_GATEWAY_CALLER_CONTEXT_PUBLIC_KEY',
          'gateway-caller-context'
        )
      }
    ]),
    callerContextSigningKey: resolveCallerContextSigningKey(
      process.env.BREVIO_GATEWAY_CALLER_CONTEXT_PRIVATE_KEY,
      environment,
      'BREVIO_GATEWAY_CALLER_CONTEXT_PRIVATE_KEY',
      'gateway-caller-context'
    ),
    callerContextIssuer: process.env.BREVIO_GATEWAY_CALLER_CONTEXT_ISSUER?.trim() || 'https://gateway.brevio.internal/caller-context',
    logSalt: process.env.BREVIO_GATEWAY_LOG_SALT?.trim() || `brevio-gateway:${environment}`,

    whatsappWebhookSecret: process.env.WHATSAPP_WEBHOOK_SECRET ?? '',
    whatsappVerifyToken: process.env.WHATSAPP_VERIFY_TOKEN ?? '',
    imessageAPIKey: process.env.IMESSAGE_API_KEY ?? '',
    temporalWebhookAPIKey: process.env.TEMPORAL_WEBHOOK_API_KEY ?? '',
    temporalWorkerBaseUrl: process.env.BREVIO_TEMPORAL_WORKER_BASE_URL?.trim() || undefined,
    temporalWorkerTimeoutMs: parsePositiveInt(process.env.BREVIO_TEMPORAL_WORKER_TIMEOUT_MS, 4000, 'BREVIO_TEMPORAL_WORKER_TIMEOUT_MS'),
    capabilityInventoryJson: process.env.BREVIO_CAPABILITY_INVENTORY_JSON?.trim() || undefined,

    idempotencyTtlMs: parsePositiveInt(process.env.BREVIO_GATEWAY_IDEMPOTENCY_TTL_MS, 24 * 60 * 60 * 1000, 'BREVIO_GATEWAY_IDEMPOTENCY_TTL_MS'),
    sessionIdleMs: parsePositiveInt(process.env.BREVIO_GATEWAY_SESSION_IDLE_MS, 4 * 60 * 60 * 1000, 'BREVIO_GATEWAY_SESSION_IDLE_MS'),

    rateLimitWindowMs: parsePositiveInt(process.env.BREVIO_GATEWAY_RATE_LIMIT_WINDOW_MS, 60 * 60 * 1000, 'BREVIO_GATEWAY_RATE_LIMIT_WINDOW_MS'),
    rateLimitMinuteWindowMs: parsePositiveInt(process.env.BREVIO_GATEWAY_RATE_LIMIT_MINUTE_WINDOW_MS, 60 * 1000, 'BREVIO_GATEWAY_RATE_LIMIT_MINUTE_WINDOW_MS'),
    rateLimitPerMinute: parsePositiveInt(process.env.BREVIO_GATEWAY_RATE_LIMIT_PER_MINUTE, 60, 'BREVIO_GATEWAY_RATE_LIMIT_PER_MINUTE'),
    rateLimitFreePerHour: parsePositiveInt(process.env.BREVIO_GATEWAY_RATE_LIMIT_FREE_PER_HOUR, 30, 'BREVIO_GATEWAY_RATE_LIMIT_FREE_PER_HOUR'),
    rateLimitProPerHour: parsePositiveInt(process.env.BREVIO_GATEWAY_RATE_LIMIT_PRO_PER_HOUR, 120, 'BREVIO_GATEWAY_RATE_LIMIT_PRO_PER_HOUR')
  };
}
