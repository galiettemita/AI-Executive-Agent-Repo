// Phase v0.7.0A — "Why?" Reply Intent + Explainability Surface route tests.
//
// Covers the new applyWhy explain-send path end-to-end through the real
// sendblue-inbound route. Asserts:
//
//   * Kill switch OFF or dep missing → bit-identical v0.5.10 behavior
//     (state-machine transition + reply_parsed audit, no explain send,
//      no brevio.explain.served audit).
//   * Kill switch ON + dep wired + matched alert with rank.reason →
//     composes "I flagged it because <reason>." (NOT field-shaped),
//     calls explainSender.send exactly once, writes a success audit
//     with empty_state=false.
//   * Kill switch ON + no matched alert → empty-state copy sent +
//     empty_state=true audit.
//   * Kill switch ON + matched alert older than the 24h lookup window
//     → empty-state copy + empty_state=true.
//   * Kill switch ON + send throws → failure audit with sanitized
//     error_code/error_reason. applyWhy's state-machine + reply_parsed
//     audit are NOT regressed.
//   * Cross-tenant LOAD-BEARING: User B's "why?" never references
//     User A's alert (their empty-state path runs); User A's alert is
//     unchanged.
//   * Privacy canary: audit detail NEVER carries sender, subject, body,
//     rank.reason text, raw alert_id, phone number, or webhook secret.
//   * STOP path still works after wiring (no regression).
//
// Test harness stays in this file (NOT folded into the existing
// sendblue-inbound.test.ts) so the v0.7.0A scope reads cleanly.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { randomUUID } from 'node:crypto';

import { InMemoryAlertStateTransitionStore } from '../core/alert-state-transitions.ts';
import { InMemoryAuditStore } from '../core/audit.ts';
import { SAFE_DEFAULT_KILL_SWITCHES, type KillSwitches } from '../core/kill-switches.ts';
import {
  EMPTY_STATE_COPY,
  EXPLAIN_SOURCE_SURFACE,
  EXPLAIN_TEMPLATE_VERSION
} from '../core/explain-renderer.ts';
import { InMemoryAlertStore } from '../memory/alerts.ts';
import { InMemoryFeedbackStore } from '../memory/feedback-events.ts';
import { InMemoryInboundReplyStore } from '../memory/inbound-replies.ts';
import { InMemoryMemorySignalStore } from '../memory/memory-signals.ts';
import { InMemoryRankResultStore } from '../memory/rank-results.ts';
import type {
  SendInput,
  SendOutcome
} from '../adapters/sendblue/client.ts';
import {
  InMemoryPhoneAllowlistStore,
  type PhoneHashConfig
} from '../security/phone-allowlist.ts';
import { type CryptoConfig } from '../security/token-crypto.ts';
import { type ReplyParseResult } from '../reply-parser/index.ts';
import {
  handleSendBlueInbound,
  type SendBlueInboundRouteDeps
} from './sendblue-inbound.ts';

const WEBHOOK_SECRET = 'shh-test-sb-webhook-secret-from-dashboard';
const WEBHOOK_SECRET_HEADER = 'sb-signing-secret';
const FOUNDER_PHONE = '+16467023459';
const FOUNDER_USER = 'founder';

interface ExplainCall {
  readonly to: string;
  readonly content: string;
}

function makeExplainSender(opts: {
  outcome?: SendOutcome;
  throwError?: Error;
} = {}): { send: (input: SendInput) => Promise<SendOutcome>; calls: ExplainCall[] } {
  const calls: ExplainCall[] = [];
  return {
    calls,
    send: async (input) => {
      calls.push({ to: input.to, content: input.content });
      if (opts.throwError !== undefined) throw opts.throwError;
      return (
        opts.outcome ??
        Object.freeze<SendOutcome>({
          kind: 'sent',
          providerStatus: 'QUEUED',
          providerMessageHandle: `mh-${randomUUID()}`,
          httpStatus: 200,
          reason: 'ok'
        })
      );
    }
  };
}

interface HarnessOverrides {
  readonly explainEnabled?: boolean;
  readonly explainSender?: { send: (input: SendInput) => Promise<SendOutcome>; calls: ExplainCall[] };
  readonly parser?: (req: { user_reply_text: string }) => Promise<ReplyParseResult>;
}

function buildHarness(overrides: HarnessOverrides = {}) {
  const inboundReplyStore = new InMemoryInboundReplyStore();
  const alertStore = new InMemoryAlertStore();
  const rankResultStore = new InMemoryRankResultStore();
  const transitions = new InMemoryAlertStateTransitionStore();
  const feedbackStore = new InMemoryFeedbackStore();
  const memoryStore = new InMemoryMemorySignalStore();
  const auditStore = new InMemoryAuditStore();

  const switches: KillSwitches = Object.freeze({
    ...SAFE_DEFAULT_KILL_SWITCHES,
    sendblue_inbound_enabled: true,
    explain_surface_enabled: overrides.explainEnabled ?? false
  });

  // Default parser routes "why" → why, "STOP" → stop, anything else → unclear.
  const defaultParser = async (req: { user_reply_text: string }): Promise<ReplyParseResult> => {
    const normalized = req.user_reply_text.trim().toLowerCase().replace(/[.!?]+$/, '');
    if (normalized === 'why' || normalized === 'why this' || normalized === 'explain') {
      return Object.freeze({
        ok: true as const,
        source: 'deterministic' as const,
        intent: 'why' as const,
        latency_ms: 0,
        input_tokens: 0,
        output_tokens: 0,
        estimated_cost_usd: 0
      });
    }
    if (normalized === 'stop' || normalized === 'unsubscribe') {
      return Object.freeze({
        ok: true as const,
        source: 'deterministic' as const,
        intent: 'stop' as const,
        latency_ms: 0,
        input_tokens: 0,
        output_tokens: 0,
        estimated_cost_usd: 0
      });
    }
    return Object.freeze({
      ok: true as const,
      source: 'classifier' as const,
      classification: Object.freeze({
        intent: 'unclear' as const,
        confidence: 0.3,
        reason: 'default',
        snooze_hint: null
      }),
      model_name: 'stub',
      prompt_version: 'stub',
      latency_ms: 0,
      input_tokens: 0,
      output_tokens: 0,
      estimated_cost_usd: 0,
      low_confidence_forced_unclear: false
    });
  };

  const senderHashKey = Buffer.alloc(32, 0xab);

  const deps: SendBlueInboundRouteDeps = {
    webhookSecret: WEBHOOK_SECRET,
    webhookSecretHeader: WEBHOOK_SECRET_HEADER,
    founderPhoneNumber: FOUNDER_PHONE,
    founderUserId: FOUNDER_USER,
    killSwitches: switches,
    inboundReplyStore,
    alertStore,
    rankResultStore,
    transitions,
    feedbackStore,
    memoryStore,
    auditStore,
    senderHashKey,
    replyParser: { parse: overrides.parser ?? defaultParser },
    explainSender: overrides.explainSender
  };
  return {
    deps,
    inboundReplyStore,
    alertStore,
    rankResultStore,
    transitions,
    feedbackStore,
    memoryStore,
    auditStore
  };
}

async function seedSentAlertWithReason(
  h: ReturnType<typeof buildHarness>,
  args: {
    userId?: string;
    alertId: string;
    reason: string;
    createdAtMsOverride?: number;
  }
): Promise<void> {
  const userId = args.userId ?? FOUNDER_USER;
  await h.rankResultStore.write({
    user_id: userId,
    message_id: `msg-${args.alertId}`,
    invocation_id: `rank-${args.alertId}`,
    model_name: 'mock',
    prompt_version: 'ranker-v0.1.0',
    label: 'important',
    score: 0.9,
    reason: args.reason,
    latency_ms: 5,
    input_tokens: 10,
    output_tokens: 5,
    estimated_cost_usd: 0
  });
  const rank = await h.rankResultStore.get(userId, `msg-${args.alertId}`);
  await h.alertStore.create({
    alert_id: args.alertId,
    user_id: userId,
    message_id: `msg-${args.alertId}`,
    rank_result_id: rank!.id,
    label: 'important',
    score: 0.9
  });
  if (args.createdAtMsOverride !== undefined) {
    // The InMemoryAlertStore freezes rows; the only way to override
    // created_at is to monkey-patch. We avoid deep store internals by
    // operating on the in-memory rows array if exposed; otherwise we
    // re-create by mutating the alertStore.recent return shape via the
    // alert.created_at field. The InMemoryAlertStore exposes a row
    // by index; we cast through unknown to mutate the row's frozen
    // shape (test-only).
    //
    // Mutating a frozen object would throw; the in-memory store row
    // is constructed with Object.freeze. Re-create by inserting a
    // sibling with a custom timestamp via direct row push.
    const rowsHandle = (h.alertStore as unknown as { rows: Array<{ alert_id: string; created_at: string }> }).rows;
    if (Array.isArray(rowsHandle)) {
      const idx = rowsHandle.findIndex((r) => r.alert_id === args.alertId);
      if (idx >= 0) {
        rowsHandle[idx] = {
          ...rowsHandle[idx]!,
          created_at: new Date(args.createdAtMsOverride).toISOString()
        };
      }
    }
  }
  for (const [from, to] of [
    ['detected', 'ranked'],
    ['ranked', 'queued_for_review'],
    ['queued_for_review', 'approved'],
    ['approved', 'sent']
  ] as const) {
    await h.transitions.write({
      alert_id: args.alertId,
      user_id: userId,
      from_state: from,
      to_state: to,
      reason: 'test seed'
    });
  }
}

function inboundBody(opts: { from?: string; content: string; messageHandle?: string }): string {
  return JSON.stringify({
    from_number: opts.from ?? FOUNDER_PHONE,
    content: opts.content,
    message_handle: opts.messageHandle ?? `sb-${randomUUID()}`
  });
}

function explainAuditsFor(audits: Array<{ action: string; result: string; detail: unknown }>) {
  return audits.filter((a) => a.action === 'brevio.explain.served');
}

/* ====================================================================== */
/* Kill switch + dep gating                                               */
/* ====================================================================== */

describe('v0.7.0A — kill switch off (default)', () => {
  it('does NOT call explainSender + does NOT write brevio.explain.served + state transition still fires', async () => {
    const explainSender = makeExplainSender();
    const h = buildHarness({ explainEnabled: false, explainSender });
    await seedSentAlertWithReason(h, {
      alertId: 'alert-killswitch-off',
      reason: 'Mark needs you to review the deck tonight.'
    });
    const body = inboundBody({ content: 'why?' });
    const r = await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(r.status, 200);
    // No outbound send.
    assert.equal(explainSender.calls.length, 0);
    // No brevio.explain.served audit at all.
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    assert.equal(explainAuditsFor(audits).length, 0);
    // State machine still walked to 'replied' (v0.5.10 baseline preserved).
    assert.equal(await h.transitions.currentState('alert-killswitch-off'), 'replied');
    // reply_parsed audit still emitted.
    assert.ok(audits.find((a) => a.action === 'fomo.sendblue.reply_parsed'));
  });
});

describe('v0.7.0A — kill switch on but explainSender dep missing', () => {
  it('silently falls through — no audit, no send', async () => {
    const h = buildHarness({ explainEnabled: true, explainSender: undefined });
    await seedSentAlertWithReason(h, {
      alertId: 'alert-no-dep',
      reason: 'Galiette needs the deck tonight.'
    });
    const body = inboundBody({ content: 'why?' });
    await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    assert.equal(explainAuditsFor(audits).length, 0);
    // Still transitioned + still reply_parsed audited.
    assert.equal(await h.transitions.currentState('alert-no-dep'), 'replied');
  });
});

/* ====================================================================== */
/* Happy path — composed explanation                                      */
/* ====================================================================== */

describe('v0.7.0A — happy path (kill switch on, matched alert, rank.reason present)', () => {
  it('composes natural-prose "I flagged it because <reason>" + sends + writes success audit', async () => {
    const explainSender = makeExplainSender();
    const h = buildHarness({ explainEnabled: true, explainSender });
    await seedSentAlertWithReason(h, {
      alertId: 'alert-happy',
      reason: 'Mark needs you to review the Q3 board deck tonight.'
    });
    const body = inboundBody({ content: 'why?' });
    const r = await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(r.status, 200);
    // Exactly one explain send.
    assert.equal(explainSender.calls.length, 1);
    const call = explainSender.calls[0]!;
    assert.equal(call.to, FOUNDER_PHONE);
    assert.equal(
      call.content,
      'I flagged it because Mark needs you to review the Q3 board deck tonight.'
    );
    // Length under 320.
    assert.ok(call.content.length <= 320);
    // No field-shaped artifacts.
    assert.equal(call.content.startsWith('From '), false);
    assert.equal(call.content.includes('—'), false);
    assert.equal(/"[^"]*"/.test(call.content), false);
    // Audit success.
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const explains = explainAuditsFor(audits);
    assert.equal(explains.length, 1);
    const e = explains[0]!;
    assert.equal(e.result, 'success');
    const detail = e.detail as Record<string, unknown>;
    assert.equal(detail.source_surface, EXPLAIN_SOURCE_SURFACE);
    assert.equal(detail.template_version, EXPLAIN_TEMPLATE_VERSION);
    assert.equal(detail.empty_state, false);
    assert.equal(detail.send_outcome_kind, 'sent');
    // alert_id_hash is non-null + hex16 + does NOT contain the raw id.
    assert.match(detail.alert_id_hash as string, /^[a-f0-9]{16}$/);
    assert.equal((detail.alert_id_hash as string).includes('alert-happy'), false);
  });
});

/* ====================================================================== */
/* Empty-state paths                                                      */
/* ====================================================================== */

describe('v0.7.0A — empty state: no eligible alert', () => {
  it('sends EMPTY_STATE_COPY + audit empty_state=true with alert_id_hash=null', async () => {
    const explainSender = makeExplainSender();
    const h = buildHarness({ explainEnabled: true, explainSender });
    // No seeded alert at all.
    const body = inboundBody({ content: 'why?' });
    await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(explainSender.calls.length, 1);
    assert.equal(explainSender.calls[0]!.content, EMPTY_STATE_COPY);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const explains = explainAuditsFor(audits);
    assert.equal(explains.length, 1);
    const detail = explains[0]!.detail as Record<string, unknown>;
    assert.equal(detail.empty_state, true);
    assert.equal(detail.alert_id_hash, null);
    assert.equal(detail.send_outcome_kind, 'sent');
  });
});

describe('v0.7.0A — empty state: matched alert older than 24h lookup window', () => {
  it('treats stale alert as no match + sends EMPTY_STATE_COPY + empty_state=true', async () => {
    const explainSender = makeExplainSender();
    const h = buildHarness({ explainEnabled: true, explainSender });
    // Seed alert 26 hours ago — outside the 24h window.
    const stale = Date.now() - 26 * 60 * 60 * 1000;
    await seedSentAlertWithReason(h, {
      alertId: 'alert-stale',
      reason: 'Some old context.',
      createdAtMsOverride: stale
    });
    const body = inboundBody({ content: 'why?' });
    await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(explainSender.calls.length, 1);
    assert.equal(explainSender.calls[0]!.content, EMPTY_STATE_COPY);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const detail = explainAuditsFor(audits)[0]!.detail as Record<string, unknown>;
    assert.equal(detail.empty_state, true);
  });
});

describe('v0.7.0A — empty state: rank_result reason is empty/whitespace', () => {
  it('composeExplanationFromReason returns null → empty-state copy', async () => {
    const explainSender = makeExplainSender();
    const h = buildHarness({ explainEnabled: true, explainSender });
    await seedSentAlertWithReason(h, {
      alertId: 'alert-emptyreason',
      reason: '   '
    });
    const body = inboundBody({ content: 'why?' });
    await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(explainSender.calls.length, 1);
    assert.equal(explainSender.calls[0]!.content, EMPTY_STATE_COPY);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const detail = explainAuditsFor(audits)[0]!.detail as Record<string, unknown>;
    assert.equal(detail.empty_state, true);
    // Even though there's a matched alert, the empty-state path sets
    // alert_id_hash=null so the audit reads consistently as "we had
    // nothing to explain".
    assert.equal(detail.alert_id_hash, null);
  });
});

/* ====================================================================== */
/* Send failure paths                                                     */
/* ====================================================================== */

describe('v0.7.0A — send throws (network/provider exception)', () => {
  it('writes failure audit with sanitized error_code + does NOT regress reply_parsed/transition', async () => {
    const explainSender = makeExplainSender({ throwError: new Error('boom') });
    const h = buildHarness({ explainEnabled: true, explainSender });
    await seedSentAlertWithReason(h, {
      alertId: 'alert-throw',
      reason: 'Mark needs the deck.'
    });
    const body = inboundBody({ content: 'why?' });
    const r = await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(r.status, 200);
    assert.equal(explainSender.calls.length, 1);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const explains = explainAuditsFor(audits);
    assert.equal(explains.length, 1);
    const e = explains[0]!;
    assert.equal(e.result, 'failure');
    const detail = e.detail as Record<string, unknown>;
    assert.equal(detail.send_outcome_kind, null);
    assert.ok(detail.error_code, 'expected sanitized error_code on throw');
    assert.ok(detail.error_reason, 'expected sanitized error_reason on throw');
    // Non-regression: state machine still walked to replied.
    assert.equal(await h.transitions.currentState('alert-throw'), 'replied');
    // reply_parsed still emitted.
    assert.ok(audits.find((a) => a.action === 'fomo.sendblue.reply_parsed'));
  });
});

describe('v0.7.0A — send returns failed outcome', () => {
  it('writes failure audit with sanitized provider error_code + error_reason', async () => {
    const explainSender = makeExplainSender({
      outcome: Object.freeze<SendOutcome>({
        kind: 'failed',
        providerStatus: 'ERROR',
        providerMessageHandle: '',
        httpStatus: 402,
        reason: 'opted_out',
        providerError: Object.freeze({
          error_message: 'OPTED_OUT',
          error_reason: 'SpamRule',
          error_code: '402'
        })
      })
    });
    const h = buildHarness({ explainEnabled: true, explainSender });
    await seedSentAlertWithReason(h, {
      alertId: 'alert-failed',
      reason: 'Some context.'
    });
    const body = inboundBody({ content: 'why?' });
    await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const detail = explainAuditsFor(audits)[0]!.detail as Record<string, unknown>;
    assert.equal((audits.find((a) => a.action === 'brevio.explain.served'))!.result, 'failure');
    assert.equal(detail.send_outcome_kind, 'failed');
    assert.ok(detail.error_code, 'sanitized error_code present');
  });
});


describe('v0.7.0A — send status unknown outcome', () => {
  it('writes failure audit and preserves the unknown provider outcome kind', async () => {
    const explainSender = makeExplainSender({
      outcome: Object.freeze<SendOutcome>({
        kind: 'send_status_unknown',
        providerStatus: 'TIMEOUT',
        providerMessageHandle: '',
        httpStatus: 504,
        reason: 'provider_timeout',
        providerError: Object.freeze({
          error_message: 'Gateway Timeout',
          error_reason: 'SendBlue timed out',
          error_code: '504'
        })
      })
    });
    const h = buildHarness({ explainEnabled: true, explainSender });
    await seedSentAlertWithReason(h, {
      alertId: 'alert-unknown',
      reason: 'Jordan needs your approval today.'
    });
    const body = inboundBody({ content: 'why?' });
    await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);

    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const explainAudit = audits.find((a) => a.action === 'brevio.explain.served');
    const detail = explainAudit!.detail as Record<string, unknown>;

    assert.equal(explainAudit!.result, 'failure');
    assert.equal(detail.send_outcome_kind, 'send_status_unknown');
    assert.ok(detail.error_code, 'sanitized error_code present');
    assert.ok(detail.error_reason, 'sanitized error_reason present');
    assert.equal(await h.transitions.currentState('alert-unknown'), 'replied');
  });
});

/* ====================================================================== */
/* Cross-tenant LOAD-BEARING                                              */
/* ====================================================================== */

describe('v0.7.0A — cross-tenant isolation LOAD-BEARING', () => {
  // Same fixtures as the multi-tenant integration test: two distinct
  // synthetic phones, two distinct user_ids. User B asks "why?";
  // User A's alert content must NEVER appear in User B's outbound
  // body or audit.
  const FOUNDER_PHONE_X = '+15550100101';
  const FRIEND_PHONE_X = '+15550100102';
  const FRIEND_USER = randomUUID();

  it("User B's why? never references User A's alert", async () => {
    const friendExplainSender = makeExplainSender();
    const inboundReplyStore = new InMemoryInboundReplyStore();
    const alertStore = new InMemoryAlertStore();
    const rankResultStore = new InMemoryRankResultStore();
    const transitions = new InMemoryAlertStateTransitionStore();
    const feedbackStore = new InMemoryFeedbackStore();
    const memoryStore = new InMemoryMemorySignalStore();
    const auditStore = new InMemoryAuditStore();

    const cryptoConfig: CryptoConfig = {
      kek: Buffer.alloc(32, 13),
      devMode: false
    };
    const phoneHashConfig: PhoneHashConfig = {
      hmacKey: Buffer.alloc(32, 91)
    };
    const phoneAllowlist = new InMemoryPhoneAllowlistStore(cryptoConfig, phoneHashConfig);
    await phoneAllowlist.setPhone(FOUNDER_USER, FOUNDER_PHONE_X);
    await phoneAllowlist.setPhone(FRIEND_USER, FRIEND_PHONE_X);

    // Seed an alert ONLY for the founder. Friend has nothing.
    await rankResultStore.write({
      user_id: FOUNDER_USER,
      message_id: 'msg-founder-1',
      invocation_id: 'rank-founder-1',
      model_name: 'mock',
      prompt_version: 'ranker-v0.1.0',
      label: 'important',
      score: 0.9,
      reason: 'FOUNDER_ONLY_SECRET Mark needs the FOUNDER_ONLY_SECRET deck.',
      latency_ms: 5,
      input_tokens: 10,
      output_tokens: 5,
      estimated_cost_usd: 0
    });
    const rank = await rankResultStore.get(FOUNDER_USER, 'msg-founder-1');
    await alertStore.create({
      alert_id: 'alert-founder-x',
      user_id: FOUNDER_USER,
      message_id: 'msg-founder-1',
      rank_result_id: rank!.id,
      label: 'important',
      score: 0.9
    });
    for (const [from, to] of [
      ['detected', 'ranked'],
      ['ranked', 'queued_for_review'],
      ['queued_for_review', 'approved'],
      ['approved', 'sent']
    ] as const) {
      await transitions.write({
        alert_id: 'alert-founder-x',
        user_id: FOUNDER_USER,
        from_state: from,
        to_state: to,
        reason: 'test seed'
      });
    }

    const stubParser = async (req: { user_reply_text: string }): Promise<ReplyParseResult> => {
      const n = req.user_reply_text.trim().toLowerCase().replace(/[.!?]+$/, '');
      if (n === 'why' || n === 'explain') {
        return Object.freeze({
          ok: true as const,
          source: 'deterministic' as const,
          intent: 'why' as const,
          latency_ms: 0,
          input_tokens: 0,
          output_tokens: 0,
          estimated_cost_usd: 0
        });
      }
      return Object.freeze({
        ok: true as const,
        source: 'classifier' as const,
        classification: Object.freeze({
          intent: 'unclear' as const,
          confidence: 0.3,
          reason: '',
          snooze_hint: null
        }),
        model_name: 'stub',
        prompt_version: 'stub',
        latency_ms: 0,
        input_tokens: 0,
        output_tokens: 0,
        estimated_cost_usd: 0,
        low_confidence_forced_unclear: false
      });
    };

    const switches: KillSwitches = Object.freeze({
      ...SAFE_DEFAULT_KILL_SWITCHES,
      sendblue_inbound_enabled: true,
      explain_surface_enabled: true
    });

    const deps: SendBlueInboundRouteDeps = {
      webhookSecret: WEBHOOK_SECRET,
      webhookSecretHeader: WEBHOOK_SECRET_HEADER,
      founderPhoneNumber: FOUNDER_PHONE_X,
      founderUserId: FOUNDER_USER,
      phoneAllowlist,
      phoneHash: phoneHashConfig,
      killSwitches: switches,
      inboundReplyStore,
      alertStore,
      rankResultStore,
      transitions,
      feedbackStore,
      memoryStore,
      auditStore,
      senderHashKey: Buffer.alloc(32, 0xab),
      replyParser: { parse: stubParser },
      explainSender: friendExplainSender
    };

    // Friend sends "why?" — should get empty-state, never see founder's reason.
    const body = JSON.stringify({
      from_number: FRIEND_PHONE_X,
      content: 'why?',
      message_handle: `sb-${randomUUID()}`
    });
    const r = await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, deps);
    assert.equal(r.status, 200);

    // Friend got the empty-state copy.
    assert.equal(friendExplainSender.calls.length, 1);
    assert.equal(friendExplainSender.calls[0]!.content, EMPTY_STATE_COPY);
    // The founder's secret reason must NOT appear in the outbound to the friend.
    assert.equal(
      friendExplainSender.calls[0]!.content.includes('FOUNDER_ONLY_SECRET'),
      false
    );

    // Founder's alert state is untouched (still 'sent' — friend's reply
    // didn't transition it).
    assert.equal(await transitions.currentState('alert-founder-x'), 'sent');

    // Audit trail: the friend's brevio.explain.served carries empty_state=true
    // and alert_id_hash=null. The founder's secret reason text must NOT appear
    // in ANY audit detail anywhere.
    const friendAudits = await auditStore.recent(FRIEND_USER, 50);
    const founderAudits = await auditStore.recent(FOUNDER_USER, 50);
    const allAudits = [...friendAudits, ...founderAudits];
    for (const a of allAudits) {
      const ds = JSON.stringify(a.detail);
      assert.equal(
        ds.includes('FOUNDER_ONLY_SECRET'),
        false,
        `audit ${a.action} (actor=${a.actor_user_id}) leaked founder secret: ${ds}`
      );
    }
    // Friend's own explain audit must be empty_state.
    const friendExplain = friendAudits.find((a) => a.action === 'brevio.explain.served');
    assert.ok(friendExplain, 'friend should have a brevio.explain.served audit');
    const detail = friendExplain!.detail as Record<string, unknown>;
    assert.equal(detail.empty_state, true);
    assert.equal(detail.alert_id_hash, null);
  });
});

/* ====================================================================== */
/* Privacy canary                                                         */
/* ====================================================================== */

describe('v0.7.0A — privacy canary on brevio.explain.served audit detail', () => {
  it('audit NEVER contains rank.reason text, raw alert_id, phone, or webhook secret', async () => {
    const explainSender = makeExplainSender();
    const h = buildHarness({ explainEnabled: true, explainSender });
    const SECRET_REASON_SUBSTRING = 'TOP_SECRET_NEEDLE_FOR_AUDIT_CHECK';
    await seedSentAlertWithReason(h, {
      alertId: 'alert-privacy-canary',
      reason: `Galiette needs ${SECRET_REASON_SUBSTRING} the deck tonight.`
    });
    const body = inboundBody({ content: 'why?' });
    await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const sysAudits = await h.auditStore.recent(null as unknown as string, 50);
    const explains = [...audits, ...sysAudits].filter((a) => a.action === 'brevio.explain.served');
    assert.ok(explains.length >= 1);
    for (const e of explains) {
      const ds = JSON.stringify(e.detail);
      assert.equal(
        ds.includes(SECRET_REASON_SUBSTRING),
        false,
        `brevio.explain.served leaked rank.reason text: ${ds}`
      );
      assert.equal(
        ds.includes('alert-privacy-canary'),
        false,
        `brevio.explain.served leaked raw alert_id: ${ds}`
      );
      assert.equal(
        ds.includes(FOUNDER_PHONE),
        false,
        `brevio.explain.served leaked phone number: ${ds}`
      );
      assert.equal(
        ds.includes(WEBHOOK_SECRET),
        false,
        `brevio.explain.served leaked webhook secret: ${ds}`
      );
    }
  });
});

/* ====================================================================== */
/* No regression: STOP path still works with explain wired                */
/* ====================================================================== */

describe('v0.7.0A — STOP path unchanged with explain surface wired', () => {
  it('STOP routes through the deterministic compliance path; no explain send', async () => {
    const explainSender = makeExplainSender();
    const h = buildHarness({ explainEnabled: true, explainSender });
    await seedSentAlertWithReason(h, {
      alertId: 'alert-stop-noregression',
      reason: 'something to skip.'
    });
    const body = inboundBody({ content: 'STOP' });
    const r = await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(r.status, 200);
    // STOP does NOT call the explain surface.
    assert.equal(explainSender.calls.length, 0);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    assert.equal(explainAuditsFor(audits).length, 0);
    // STOP-recorded audit fires.
    assert.ok(audits.find((a) => a.action === 'fomo.sendblue.stop_recorded'));
  });
});
