import type http from 'node:http';

import type { ConsentCategory, ConsentState } from '@brevio/shared';

import type { AuditStore } from './audit.js';
import type { AuthenticatedContext } from './auth-middleware.js';
import type { ConsentStore } from './consent-store.js';
import type { PendingMessageStore } from './pending-message-store.js';
import type { RateLimiter } from './rate-limit.js';
import type { TokenStore } from './token-store.js';

const VALID_CATEGORIES: readonly ConsentCategory[] = ['email', 'money', 'health'];
const VALID_STATES: readonly ConsentState[] = ['granted', 'revoked', 'snoozed'];
const VALID_SOURCES = ['inline_prompt', 'settings', 'oauth_callback', 'api', 'admin'] as const;

export interface ConsentRouteDeps {
  consentStore: ConsentStore;
  auditStore: AuditStore;
  rateLimiter: RateLimiter;
  tokenStore: TokenStore;
  pendingMessageStore: PendingMessageStore;
}

interface RouteContext {
  auth: AuthenticatedContext;
  ip: string | null;
  userAgent: string | null;
  requestId: string;
  method: string;
  path: string;
}

function readJSON(req: http.IncomingMessage, maxBytes = 64 * 1024): Promise<Record<string, unknown>> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    let total = 0;
    req.on('data', (chunk: Buffer) => {
      total += chunk.length;
      if (total > maxBytes) {
        reject(new Error('payload_too_large'));
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => {
      if (chunks.length === 0) {
        resolve({});
        return;
      }
      try {
        const parsed = JSON.parse(Buffer.concat(chunks).toString('utf8'));
        if (typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)) {
          resolve(parsed as Record<string, unknown>);
        } else {
          reject(new Error('invalid_json_object'));
        }
      } catch {
        reject(new Error('invalid_json'));
      }
    });
    req.on('error', reject);
  });
}

function sendJSON(res: http.ServerResponse, status: number, body: Record<string, unknown>): void {
  res.writeHead(status, { 'content-type': 'application/json' });
  res.end(JSON.stringify(body));
}

function sendError(res: http.ServerResponse, status: number, code: string, message: string, requestId: string): void {
  sendJSON(res, status, { error: { code, message, request_id: requestId } });
}

function rateLimitOrError(
  res: http.ServerResponse,
  ctx: RouteContext,
  deps: ConsentRouteDeps,
  endpoint: string
): boolean {
  const decision = deps.rateLimiter.consume(endpoint, ctx.auth.user_id);
  if (decision.allowed) return true;
  res.setHeader('Retry-After', Math.ceil(decision.retryAfterMs / 1000).toString());
  sendError(res, 429, 'rate_limited', `slow down — retry after ${decision.retryAfterMs}ms`, ctx.requestId);
  return false;
}

function validateCategory(value: unknown): value is ConsentCategory {
  return typeof value === 'string' && (VALID_CATEGORIES as readonly string[]).includes(value);
}

function validateState(value: unknown): value is ConsentState {
  return typeof value === 'string' && (VALID_STATES as readonly string[]).includes(value);
}

function validateSource(value: unknown): value is typeof VALID_SOURCES[number] {
  return typeof value === 'string' && (VALID_SOURCES as readonly string[]).includes(value);
}

export async function handleGetOnboarding(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  ctx: RouteContext,
  deps: ConsentRouteDeps
): Promise<void> {
  void req;
  const dismissed_at = await deps.consentStore.getOnboardingDismissed(ctx.auth.user_id);
  sendJSON(res, 200, { card_dismissed_at: dismissed_at });
}

export async function handleDismissOnboarding(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  ctx: RouteContext,
  deps: ConsentRouteDeps
): Promise<void> {
  void req;
  await deps.consentStore.markOnboardingDismissed(ctx.auth.user_id);
  await deps.auditStore.write({
    actor_user_id: ctx.auth.user_id,
    actor_ip: ctx.ip,
    actor_user_agent: ctx.userAgent,
    action: 'onboarding.dismissed',
    target: null,
    result: 'success',
    detail: null
  });
  sendJSON(res, 200, { ok: true });
}

export async function handleGetConsent(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  ctx: RouteContext,
  deps: ConsentRouteDeps
): Promise<void> {
  void req;
  const states = await deps.consentStore.getCategoryStates(ctx.auth.user_id);
  sendJSON(res, 200, { consent: states });
}

export async function handlePostConsent(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  ctx: RouteContext,
  deps: ConsentRouteDeps
): Promise<void> {
  if (!rateLimitOrError(res, ctx, deps, 'POST /api/v1/me/consent')) return;
  let body: Record<string, unknown>;
  try {
    body = await readJSON(req);
  } catch (err) {
    sendError(res, 400, 'invalid_body', err instanceof Error ? err.message : 'unknown', ctx.requestId);
    return;
  }
  const { category, state, source, session_id } = body;
  if (!validateCategory(category)) {
    sendError(res, 400, 'invalid_category', 'expected email | money | health', ctx.requestId);
    return;
  }
  if (!validateState(state)) {
    sendError(res, 400, 'invalid_state', 'expected granted | revoked | snoozed', ctx.requestId);
    return;
  }
  if (!validateSource(source)) {
    sendError(res, 400, 'invalid_source', 'expected inline_prompt | settings | oauth_callback | api | admin', ctx.requestId);
    return;
  }

  const sess = typeof session_id === 'string' ? session_id : ctx.auth.session_id;
  await deps.consentStore.setCategoryState({
    user_id: ctx.auth.user_id,
    category,
    state,
    source,
    session_id: state === 'snoozed' ? sess : undefined
  });

  const action = state === 'granted'
    ? 'consent.grant'
    : state === 'snoozed'
      ? 'consent.snooze'
      : 'consent.revoke';
  await deps.auditStore.write({
    actor_user_id: ctx.auth.user_id,
    actor_ip: ctx.ip,
    actor_user_agent: ctx.userAgent,
    action,
    target: `category:${category}`,
    result: 'success',
    detail: { source, session_id: state === 'snoozed' ? sess : undefined }
  });

  sendJSON(res, 200, { ok: true, category, state });
}

export async function handlePostConsentRevoke(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  ctx: RouteContext,
  deps: ConsentRouteDeps
): Promise<void> {
  if (!rateLimitOrError(res, ctx, deps, 'POST /api/v1/me/consent/revoke')) return;
  let body: Record<string, unknown>;
  try {
    body = await readJSON(req);
  } catch (err) {
    sendError(res, 400, 'invalid_body', err instanceof Error ? err.message : 'unknown', ctx.requestId);
    return;
  }
  const { category } = body;
  if (!validateCategory(category)) {
    sendError(res, 400, 'invalid_category', 'expected email | money | health', ctx.requestId);
    return;
  }
  const outcome = await deps.consentStore.revokeCategory(ctx.auth.user_id, category);

  // Cascade: actually delete tokens for each provider tied to this category.
  // Per plan §6.5: local deletion is the source of truth — we don't block on
  // upstream provider revoke (that lives in oauth-exchange.revokeAtProvider and
  // is best-effort).
  const disconnected: string[] = [];
  for (const provider of outcome.providers_to_disconnect) {
    if (await deps.tokenStore.has(ctx.auth.user_id, provider)) {
      await deps.tokenStore.delete(ctx.auth.user_id, provider);
      disconnected.push(provider);
      await deps.auditStore.write({
        actor_user_id: ctx.auth.user_id,
        actor_ip: ctx.ip,
        actor_user_agent: ctx.userAgent,
        action: 'oauth.disconnect',
        target: `provider:${provider}`,
        result: 'success',
        detail: { trigger: 'consent.revoke', category }
      });
    }
  }

  await deps.auditStore.write({
    actor_user_id: ctx.auth.user_id,
    actor_ip: ctx.ip,
    actor_user_agent: ctx.userAgent,
    action: 'consent.revoke',
    target: `category:${category}`,
    result: 'success',
    detail: {
      providers_to_disconnect: outcome.providers_to_disconnect,
      providers_actually_disconnected: disconnected
    }
  });
  sendJSON(res, 200, {
    ok: true,
    category,
    providers_to_disconnect: outcome.providers_to_disconnect,
    providers_disconnected: disconnected
  });
}

export async function handleGetEnabledSkills(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  ctx: RouteContext,
  deps: ConsentRouteDeps
): Promise<void> {
  void req;
  if (!rateLimitOrError(res, ctx, deps, 'GET /api/v1/me/skills/enabled')) return;
  const result = await deps.consentStore.computeEnabledSkills(ctx.auth.user_id, ctx.auth.session_id);
  sendJSON(res, 200, {
    enabled_skills: result.enabledSkills,
    by_category: result.byCategory,
    source: result.source
  });
}

export async function handleGetPendingMessage(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  ctx: RouteContext,
  deps: ConsentRouteDeps,
  pendingId: string
): Promise<void> {
  void req;
  const peek = await deps.pendingMessageStore.peek(pendingId, ctx.auth.user_id);
  if (!peek) {
    sendError(res, 404, 'pending_not_found', 'pending message not found or expired', ctx.requestId);
    return;
  }
  sendJSON(res, 200, {
    pending_message_id: peek.pending_message_id,
    original_text: peek.original_text,
    channel: peek.channel,
    session_id: peek.session_id
  });
}

export async function handleConsumePendingMessage(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  ctx: RouteContext,
  deps: ConsentRouteDeps,
  pendingId: string
): Promise<void> {
  void req;
  const consumed = await deps.pendingMessageStore.consume(pendingId, ctx.auth.user_id);
  if (!consumed) {
    sendError(res, 404, 'pending_not_found', 'pending message not found, expired, or already consumed', ctx.requestId);
    return;
  }
  sendJSON(res, 200, {
    pending_message_id: consumed.pending_message_id,
    original_text: consumed.original_text,
    channel: consumed.channel,
    session_id: consumed.session_id
  });
}

export async function handleGetAudit(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  ctx: RouteContext,
  deps: ConsentRouteDeps
): Promise<void> {
  void req;
  const entries = await deps.auditStore.recent(ctx.auth.user_id, 100);
  sendJSON(res, 200, { entries });
}

export type { RouteContext };
export { sendError, sendJSON };
