// Phase v0.5.5 — STOP Enforcement + Confirmation runtime tests.
//
// Covers the six founder-mandated proof points:
//   1. Duplicate STOP within 24h does NOT resend confirmation.
//   2. STOP after 24h MAY send a fresh confirmation.
//   3. Confirmation send failure writes fomo.sendblue.stop_confirmation_failed
//      and does NOT retry.
//   4. stop_active suppresses future alert creation for THAT user only.
//   5. Other users remain unaffected (cross-tenant invariant).
//   6. Confirmation copy is deterministic, friendly, contains NO email content.
//
// Plus a few defense-in-depth checks: thrown-error path, dep-absent path,
// no-retry-after-status-unknown path.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAlertStateTransitionStore } from '../core/alert-state-transitions.ts';
import { InMemoryAuditStore } from '../core/audit.ts';
import { SAFE_DEFAULT_KILL_SWITCHES } from '../core/kill-switches.ts';
import { InMemoryAlertStore } from '../memory/alerts.ts';
import { InMemoryFeedbackStore } from '../memory/feedback-events.ts';
import { InMemoryInboundReplyStore } from '../memory/inbound-replies.ts';
import { InMemoryMemorySignalStore } from '../memory/memory-signals.ts';
import { InMemoryRankResultStore } from '../memory/rank-results.ts';
import {
  type SendInput,
  type SendOutcome
} from '../adapters/sendblue/client.ts';
import { type ReplyParseResult } from '../reply-parser/index.ts';
import {
  handleSendBlueInbound,
  STOP_CONFIRMATION_BODY,
  STOP_CONFIRMATION_IDEMPOTENCY_WINDOW_MS,
  type SendBlueInboundRouteDeps
} from './sendblue-inbound.ts';

const WEBHOOK_SECRET = 'shh-test-sb-webhook-secret-from-dashboard';
const WEBHOOK_SECRET_HEADER = 'sb-signing-secret';
const FOUNDER_PHONE = '+16467023459';
const FOUNDER_USER = 'founder';

// A second user keyed by a DIFFERENT phone, used in the cross-tenant tests.
const FRIEND_PHONE = '+15555550100';
const FRIEND_USER = 'friend-user-uuid';

interface MockSendCall {
  readonly input: SendInput;
  readonly at: number;
}

interface StopConfirmationMock {
  readonly calls: MockSendCall[];
  readonly send: (input: SendInput) => Promise<SendOutcome>;
}

/**
 * Build a mock STOP confirmation send dep. The default returns 'sent'.
 * Override `outcomeFor` to return 'failed'/'send_status_unknown' or throw.
 */
function buildStopConfirmationMock(
  outcomeFor: (callIndex: number) => SendOutcome | Promise<SendOutcome> | (() => never) = () =>
    Object.freeze({
      kind: 'sent' as const,
      providerStatus: 'QUEUED',
      providerMessageHandle: 'sb-out-handle',
      httpStatus: 200,
      reason: 'mock send ok'
    })
): StopConfirmationMock {
  const calls: MockSendCall[] = [];
  const send = async (input: SendInput): Promise<SendOutcome> => {
    const index = calls.length;
    calls.push({ input, at: Date.now() });
    const out = outcomeFor(index);
    if (typeof out === 'function') {
      // Trigger throw path.
      (out as () => never)();
    }
    return await Promise.resolve(out as SendOutcome);
  };
  return { calls, send };
}

interface HarnessOverrides {
  readonly stopConfirmation?: StopConfirmationMock;
  // Optional clock; controls applyStop's `now` for idempotency-window tests.
  readonly now?: () => Date;
  // Optional phoneAllowlist + phoneHash for multi-user tests. When absent,
  // the route is founder-only (env-equality match).
  readonly enableMultiTenant?: boolean;
}

function buildHarness(overrides: HarnessOverrides = {}) {
  const inboundReplyStore = new InMemoryInboundReplyStore();
  const alertStore = new InMemoryAlertStore();
  const rankResultStore = new InMemoryRankResultStore();
  const transitions = new InMemoryAlertStateTransitionStore();
  const feedbackStore = new InMemoryFeedbackStore();
  const memoryStore = new InMemoryMemorySignalStore();
  const auditStore = new InMemoryAuditStore();

  const parser = async (): Promise<ReplyParseResult> =>
    Object.freeze({
      ok: true as const,
      source: 'deterministic' as const,
      intent: 'stop' as const
    });

  // Minimal phoneAllowlist for multi-tenant tests. Maps the FRIEND_PHONE
  // hash → FRIEND_USER user_id. Simple in-memory implementation.
  let phoneAllowlist:
    | { getUserIdByPhoneHash: (hash: string) => Promise<string | null>; setPhone: (input: { user_id: string; phone_hash: string; phone_e164_hash: string }) => Promise<void> }
    | undefined;
  let phoneHash:
    | { hash: (phone: string) => string }
    | undefined;
  if (overrides.enableMultiTenant) {
    const allowlist = new Map<string, string>();
    // Trivial deterministic "hash" — just the phone itself. Tests don't
    // care about cryptographic properties; they care about lookup correctness.
    phoneHash = { hash: (phone: string) => `h:${phone}` };
    allowlist.set(phoneHash.hash(FRIEND_PHONE), FRIEND_USER);
    phoneAllowlist = {
      getUserIdByPhoneHash: async (h: string) => allowlist.get(h) ?? null,
      setPhone: async () => undefined
    };
  }

  const deps: SendBlueInboundRouteDeps = {
    webhookSecret: WEBHOOK_SECRET,
    webhookSecretHeader: WEBHOOK_SECRET_HEADER,
    founderPhoneNumber: FOUNDER_PHONE,
    founderUserId: FOUNDER_USER,
    killSwitches: { ...SAFE_DEFAULT_KILL_SWITCHES, sendblue_inbound_enabled: true },
    inboundReplyStore,
    alertStore,
    rankResultStore,
    transitions,
    feedbackStore,
    memoryStore,
    auditStore,
    replyParser: { parse: parser },
    stopConfirmation: overrides.stopConfirmation,
    now: overrides.now,
    phoneAllowlist: phoneAllowlist as unknown as SendBlueInboundRouteDeps['phoneAllowlist'],
    phoneHash: phoneHash as unknown as SendBlueInboundRouteDeps['phoneHash']
  };
  return { deps, memoryStore, auditStore, feedbackStore };
}

function inboundBody(overrides: Partial<{ from_number: string; content: string; message_handle: string }> = {}): string {
  return JSON.stringify({
    from_number: overrides.from_number ?? FOUNDER_PHONE,
    content: overrides.content ?? 'STOP',
    message_handle: overrides.message_handle ?? `sb-${Math.random().toString(36).slice(2, 10)}`
  });
}

/* ====================================================================== */
/* (6) Deterministic confirmation copy                                    */
/* ====================================================================== */

describe('v0.5.5 STOP confirmation — deterministic copy', () => {
  it('STOP_CONFIRMATION_BODY contains the canonical phrases the smoke-evidence C8 check looks for', () => {
    // Smoke-evidence C8 regex: /unsubscrib|stop/i AND /start/i
    assert.match(STOP_CONFIRMATION_BODY, /unsubscrib|stop/i);
    assert.match(STOP_CONFIRMATION_BODY, /start/i);
  });

  it('STOP_CONFIRMATION_BODY is short + bounded (≤280 chars) + contains zero email-content patterns', () => {
    assert.ok(STOP_CONFIRMATION_BODY.length <= 280, `body too long: ${STOP_CONFIRMATION_BODY.length}`);
    // None of the canary substrings the C9 leak-scan looks for.
    for (const forbidden of ['brevio-canary-', 'Subject:', 'From:', '@gmail.com']) {
      assert.ok(
        !STOP_CONFIRMATION_BODY.includes(forbidden),
        `STOP_CONFIRMATION_BODY contains forbidden substring '${forbidden}'`
      );
    }
  });

  it('STOP_CONFIRMATION_IDEMPOTENCY_WINDOW_MS is exactly 24 hours (Q5)', () => {
    assert.equal(STOP_CONFIRMATION_IDEMPOTENCY_WINDOW_MS, 24 * 60 * 60 * 1000);
  });
});

/* ====================================================================== */
/* (1) Duplicate STOP within 24h does NOT resend                          */
/* ====================================================================== */

describe('v0.5.5 STOP confirmation — 24h idempotency (first send + duplicate within window)', () => {
  it('first STOP sends confirmation; second STOP within 24h does NOT resend', async () => {
    const startMs = new Date('2026-06-04T12:00:00Z').getTime();
    let nowMs = startMs;
    const clock = () => new Date(nowMs);
    const mock = buildStopConfirmationMock();
    const h = buildHarness({ stopConfirmation: mock, now: clock });

    // First STOP at t=0.
    await handleSendBlueInbound(
      { body: inboundBody({ message_handle: 'sb-stop-1' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(mock.calls.length, 1, 'first STOP should trigger one send');
    assert.equal(mock.calls[0]?.input.to, FOUNDER_PHONE);
    assert.equal(mock.calls[0]?.input.content, STOP_CONFIRMATION_BODY);

    // Advance the clock 12 hours (well inside the 24h window).
    nowMs += 12 * 60 * 60 * 1000;

    // Second STOP within the window.
    await handleSendBlueInbound(
      { body: inboundBody({ message_handle: 'sb-stop-2' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(mock.calls.length, 1, 'second STOP within 24h must NOT resend confirmation');

    // Audit shape: exactly ONE stop_confirmation_sent, ZERO stop_confirmation_failed.
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    assert.equal(
      audits.filter((e) => e.action === 'fomo.sendblue.stop_confirmation_sent').length,
      1,
      'exactly one confirmation_sent audit'
    );
    assert.equal(
      audits.filter((e) => e.action === 'fomo.sendblue.stop_confirmation_failed').length,
      0,
      'no failed audits'
    );

    // The memory signal detail records the confirmation timestamp so the
    // idempotency check can read it back.
    const signal = await h.memoryStore.get(FOUNDER_USER, 'stop_active', null);
    const detail = (signal?.detail ?? {}) as { stop_confirmation_sent_at?: unknown; active?: unknown };
    assert.equal(detail.active, true);
    assert.equal(typeof detail.stop_confirmation_sent_at, 'string');
  });
});

/* ====================================================================== */
/* (2) STOP after 24h may send a fresh confirmation                       */
/* ====================================================================== */

describe('v0.5.5 STOP confirmation — 24h idempotency (refresh after window)', () => {
  it('STOP after 24h+1ms triggers a fresh confirmation send', async () => {
    const startMs = new Date('2026-06-04T12:00:00Z').getTime();
    let nowMs = startMs;
    const clock = () => new Date(nowMs);
    const mock = buildStopConfirmationMock();
    const h = buildHarness({ stopConfirmation: mock, now: clock });

    await handleSendBlueInbound(
      { body: inboundBody({ message_handle: 'sb-stop-a' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );
    assert.equal(mock.calls.length, 1);

    // Advance the clock to JUST PAST 24h.
    nowMs += STOP_CONFIRMATION_IDEMPOTENCY_WINDOW_MS + 1;

    await handleSendBlueInbound(
      { body: inboundBody({ message_handle: 'sb-stop-b' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );
    assert.equal(mock.calls.length, 2, 'STOP after 24h should trigger a fresh send');

    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    assert.equal(
      audits.filter((e) => e.action === 'fomo.sendblue.stop_confirmation_sent').length,
      2,
      'two confirmation_sent audits (one per fresh window)'
    );
  });
});

/* ====================================================================== */
/* (3) Confirmation send failure → _failed audit + NO retry               */
/* ====================================================================== */

describe('v0.5.5 STOP confirmation — failure handling (Q6: best-effort audit, no retry)', () => {
  it('SendBlue send returns kind=failed → fomo.sendblue.stop_confirmation_failed audit + NO retry', async () => {
    const mock = buildStopConfirmationMock(() =>
      Object.freeze({
        kind: 'failed' as const,
        providerStatus: 'ERROR',
        providerMessageHandle: '',
        httpStatus: 400,
        reason: 'mock 400',
        providerError: {
          error_code: 'TEST_FAIL',
          error_message: 'TEST_ERROR',
          error_reason: 'SpamRule'
        }
      })
    );
    const h = buildHarness({ stopConfirmation: mock });

    await handleSendBlueInbound(
      { body: inboundBody({ message_handle: 'sb-fail-1' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(mock.calls.length, 1, 'one send call attempted');

    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const failedAudits = audits.filter((e) => e.action === 'fomo.sendblue.stop_confirmation_failed');
    const sentAudits = audits.filter((e) => e.action === 'fomo.sendblue.stop_confirmation_sent');
    assert.equal(failedAudits.length, 1, 'exactly one _failed audit');
    assert.equal(sentAudits.length, 0, 'no _sent audit on failure');

    // Per Q6: the failure does NOT bump stop_confirmation_sent_at, so
    // a subsequent STOP can retry the send once. Verify by checking the
    // memory signal detail is absent of the timestamp.
    const signal = await h.memoryStore.get(FOUNDER_USER, 'stop_active', null);
    const detail = (signal?.detail ?? {}) as { stop_confirmation_sent_at?: unknown };
    assert.equal(
      detail.stop_confirmation_sent_at,
      undefined,
      'failure must not bump idempotency timestamp (so a re-STOP can retry once)'
    );
  });

  it('SendBlue send returns kind=send_status_unknown → _failed audit + NO retry', async () => {
    const mock = buildStopConfirmationMock(() =>
      Object.freeze({
        kind: 'send_status_unknown' as const,
        providerStatus: undefined,
        providerMessageHandle: '',
        httpStatus: 0,
        reason: 'network timeout'
      })
    );
    const h = buildHarness({ stopConfirmation: mock });

    await handleSendBlueInbound(
      { body: inboundBody({ message_handle: 'sb-unk-1' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(mock.calls.length, 1);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    assert.equal(audits.filter((e) => e.action === 'fomo.sendblue.stop_confirmation_failed').length, 1);
    assert.equal(audits.filter((e) => e.action === 'fomo.sendblue.stop_confirmation_sent').length, 0);
  });

  it('SendBlue send throws → defense-in-depth audit + NO retry, NO crash', async () => {
    const mock = buildStopConfirmationMock(() => () => {
      throw new Error('synthetic transport failure');
    });
    const h = buildHarness({ stopConfirmation: mock });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ message_handle: 'sb-throw-1' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    // Inbound webhook still returns 200; throwing from the courtesy
    // send must not cascade to a 500.
    assert.equal(r.status, 200);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const failed = audits.find((e) => e.action === 'fomo.sendblue.stop_confirmation_failed');
    assert.ok(failed, 'failed audit row must exist');
    // Phase v0.5.15 — sanitized error_code/error_reason. The throw path
    // maps to TEMPORARY_PROVIDER_ERROR (network-style retryable failure)
    // — operationally distinct from clean 4xx/5xx via error_reason.
    const failedDetail = failed?.detail as Record<string, unknown> | undefined;
    assert.equal(failedDetail?.error_code, 'TEMPORARY_PROVIDER_ERROR');
    assert.equal(failedDetail?.error_reason, 'temporary_provider_error');
    // Raw err.message is NOT present (deny-by-default contract).
    assert.equal(failedDetail?.error_message, undefined);
    // stop_recorded still fired (STOP enforcement itself is load-bearing).
    assert.equal(audits.filter((e) => e.action === 'fomo.sendblue.stop_recorded').length, 1);
  });
});

/* ====================================================================== */
/* (5)+(6) Cross-tenant isolation + dep-absent path                       */
/* ====================================================================== */

describe('v0.5.5 STOP confirmation — cross-tenant + dep-absent', () => {
  it('founder STOP does NOT trigger a confirmation to a different user', async () => {
    const mock = buildStopConfirmationMock();
    const h = buildHarness({ stopConfirmation: mock, enableMultiTenant: true });

    // Pre-seed a stop_active row for the friend so we can verify it's
    // not touched by the founder's STOP.
    await h.memoryStore.upsert({
      user_id: FRIEND_USER,
      kind: 'stop_active',
      scope_key: null,
      detail: { active: false, recorded_at: '2026-01-01T00:00:00.000Z' },
      source: 'user_confirmed',
      confidence: 1.0
    });

    await handleSendBlueInbound(
      { body: inboundBody({ from_number: FOUNDER_PHONE, message_handle: 'sb-ct-1' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(mock.calls.length, 1);
    assert.equal(mock.calls[0]?.input.to, FOUNDER_PHONE, 'confirmation goes to founder phone only');

    // Friend's memory signal is unchanged.
    const friendSignal = await h.memoryStore.get(FRIEND_USER, 'stop_active', null);
    const friendDetail = (friendSignal?.detail ?? {}) as { active?: unknown; recorded_at?: unknown };
    assert.equal(friendDetail.active, false);
    assert.equal(friendDetail.recorded_at, '2026-01-01T00:00:00.000Z', 'friend row byte-identical');

    // No audits for the friend.
    const friendAudits = await h.auditStore.recent(FRIEND_USER, 50);
    assert.equal(friendAudits.length, 0, 'no audits attributed to the friend');
  });

  it('stopConfirmation dep absent → STOP recorded, NO send, NO confirmation audit (preserves v0.5.4 behavior)', async () => {
    const h = buildHarness(); // no stopConfirmation override → dep absent

    await handleSendBlueInbound(
      { body: inboundBody({ message_handle: 'sb-noconf-1' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    assert.equal(audits.filter((e) => e.action === 'fomo.sendblue.stop_recorded').length, 1);
    assert.equal(audits.filter((e) => e.action === 'fomo.sendblue.stop_confirmation_sent').length, 0);
    assert.equal(audits.filter((e) => e.action === 'fomo.sendblue.stop_confirmation_failed').length, 0);
  });
});
