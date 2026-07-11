import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAlertStateTransitionStore } from '../core/alert-state-transitions.ts';
import { InMemoryAuditStore } from '../core/audit.ts';
import { SAFE_DEFAULT_KILL_SWITCHES, type KillSwitches } from '../core/kill-switches.ts';
import { InMemoryAlertStore } from '../memory/alerts.ts';
import { InMemoryFeedbackStore } from '../memory/feedback-events.ts';
import { InMemoryInboundReplyStore } from '../memory/inbound-replies.ts';
import { InMemoryMemorySignalStore } from '../memory/memory-signals.ts';
import {
  InMemoryTypedMemoryStore,
  type NewTypedMemoryRow
} from '../memory/typed-memory.ts';
import { recallVisibleExplicitPreference } from '../memory/typed-memory-visible-recall.ts';
import { InMemoryRankResultStore } from '../memory/rank-results.ts';
import {
  type ReplyParseResult,
  type ReplyParserRequest
} from '../reply-parser/index.ts';
import {
  createSendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate,
  dispatchSendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate,
  handleSendBlueInbound,
  parseSendBlueInboundVisibleMemoryExactCommandContext,
  renderSendBlueInboundVisibleMemoryCommandReplyText,
  type SendBlueInboundRouteDeps,
  type SendBlueInboundVisibleMemoryCommandContext,
  type SendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate,
  type SendBlueInboundVisibleMemoryCommandReplyDeliveryDispatchResult,
  type SendBlueInboundVisibleMemoryCommandReplyText,
  type SendBlueInboundVisibleMemoryCommandResponseEnvelope
} from './sendblue-inbound.ts';

const WEBHOOK_SECRET = 'shh-test-sb-webhook-secret-from-dashboard';
const WEBHOOK_SECRET_HEADER = 'sb-signing-secret';
const FOUNDER_PHONE = '+16467023459';
const FOUNDER_USER = 'founder';


interface HarnessOverrides {
  readonly killSwitches?: KillSwitches;
  readonly visibleMemoryCommand?: SendBlueInboundRouteDeps['visibleMemoryCommand'];
  // Stub parser. Defaults to returning unclear (so the route's path
  // for unmatched/unclear can be tested without a router).
  readonly parser?: (req: ReplyParserRequest) => Promise<ReplyParseResult>;
}

function buildHarness(overrides: HarnessOverrides = {}) {
  const inboundReplyStore = new InMemoryInboundReplyStore();
  const alertStore = new InMemoryAlertStore();
  const rankResultStore = new InMemoryRankResultStore();
  const transitions = new InMemoryAlertStateTransitionStore();
  const feedbackStore = new InMemoryFeedbackStore();
  const memoryStore = new InMemoryMemorySignalStore();
  const auditStore = new InMemoryAuditStore();

  const switches: KillSwitches =
    overrides.killSwitches ??
    Object.freeze({ ...SAFE_DEFAULT_KILL_SWITCHES, sendblue_inbound_enabled: true });

  const parser =
    overrides.parser ??
    (async (): Promise<ReplyParseResult> =>
      Object.freeze({
        ok: true as const,
        source: 'classifier' as const,
        classification: Object.freeze({
          intent: 'unclear' as const,
          confidence: 0.3,
          reason: 'default test parser',
          snooze_hint: null
        }),
        model_name: 'stub',
        prompt_version: 'reply-parser-v0.1.0',
        latency_ms: 1,
        input_tokens: 1,
        output_tokens: 1,
        estimated_cost_usd: 0,
        low_confidence_forced_unclear: false
      }));

  // Phase v0.5.10 — synthetic 32-byte HMAC key for the v0.5.9 applyFeedback
  // consumer arm (ignore_sender → sender_feedback_ignored). Tests don't
  // load real BREVIO_SENDER_HASH_KEY; this fixture is sufficient for
  // exercising the routing module's consumer-arm code path.
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
    replyParser: { parse: parser },
    visibleMemoryCommand: overrides.visibleMemoryCommand
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

// Build a body that looks like a SendBlue inbound webhook payload.
function inboundBody(overrides: Partial<{ from_number: string; content: string; message_handle: string }> = {}): string {
  return JSON.stringify({
    from_number: overrides.from_number ?? FOUNDER_PHONE,
    content: overrides.content ?? 'tomorrow',
    message_handle: overrides.message_handle ?? `sb-${Math.random().toString(36).slice(2, 10)}`
  });
}

async function seedSentAlert(
  h: ReturnType<typeof buildHarness>,
  alertId = 'alert-1'
): Promise<string> {
  await h.rankResultStore.write({
    user_id: FOUNDER_USER,
    message_id: `msg-${alertId}`,
    invocation_id: `rank-${alertId}`,
    model_name: 'mock',
    prompt_version: 'ranker-v0.1.0',
    label: 'important',
    score: 0.9,
    reason: 'test seed',
    latency_ms: 5,
    input_tokens: 10,
    output_tokens: 5,
    estimated_cost_usd: 0
  });
  const rank = await h.rankResultStore.get(FOUNDER_USER, `msg-${alertId}`);
  await h.alertStore.create({
    alert_id: alertId,
    user_id: FOUNDER_USER,
    message_id: `msg-${alertId}`,
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
    await h.transitions.write({
      alert_id: alertId,
      user_id: FOUNDER_USER,
      from_state: from,
      to_state: to,
      reason: 'test seed'
    });
  }
  return alertId;
}

/* ====================================================================== */
/* Kill switch                                                            */
/* ====================================================================== */

describe('handleSendBlueInbound — kill switch off', () => {
  it('returns 200 + audits kill_switch_off + does NOT process the payload', async () => {
    const h = buildHarness({
      killSwitches: { ...SAFE_DEFAULT_KILL_SWITCHES, sendblue_inbound_enabled: false }
    });
    const body = inboundBody();
    const r = await handleSendBlueInbound(
      { body, secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );
    assert.equal(r.status, 200);
    const events = await h.auditStore.recent(FOUNDER_USER, 50);
    const sysEvents = await h.auditStore.recent(null as unknown as string, 50);
    const all = [...events, ...sysEvents];
    assert.ok(all.find((e) => e.action === 'fomo.sendblue.kill_switch_off'));
    // Did NOT touch state machine, feedback, memory, or dedup store.
    assert.equal(await h.inboundReplyStore.count(FOUNDER_USER), 0);
    assert.equal(await h.feedbackStore.countByKind(FOUNDER_USER, 'stop'), 0);
  });
});

/* ====================================================================== */
/* Webhook secret verification (SendBlue uses shared-secret-in-header)    */
/*                                                                        */
/* Per docs.sendblue.com/getting-started/webhooks:                        */
/*   "When you configure a secret, Sendblue will include it in the        */
/*    webhook request headers, allowing you to verify that the request    */
/*    is genuinely from Sendblue."                                        */
/* NOT HMAC over body. Plain equality with timing-safe compare.           */
/* ====================================================================== */

describe('handleSendBlueInbound — webhook secret auth failures (fail-closed)', () => {
  it('returns 401 + audits signature_invalid + error_code=secret_mismatch on wrong secret', async () => {
    const h = buildHarness();
    const body = inboundBody();
    const r = await handleSendBlueInbound(
      { body, secretHeaderValue: 'completely-wrong-secret-value' },
      h.deps
    );
    assert.equal(r.status, 401);
    const audits = await h.auditStore.recent(null as unknown as string, 50);
    const failAudit = audits.find((e) => e.action === 'fomo.sendblue.signature_invalid');
    assert.ok(failAudit);
    assert.equal((failAudit?.detail as { error_code: string }).error_code, 'secret_mismatch');
    // Did NOT process the payload past auth verification.
    assert.equal(await h.inboundReplyStore.count(FOUNDER_USER), 0);
  });

  it('returns 401 + error_code=missing_header when the secret header is absent', async () => {
    const h = buildHarness();
    const r = await handleSendBlueInbound(
      { body: inboundBody(), secretHeaderValue: '' },
      h.deps
    );
    assert.equal(r.status, 401);
    const audits = await h.auditStore.recent(null as unknown as string, 50);
    const failAudit = audits.find((e) => e.action === 'fomo.sendblue.signature_invalid');
    assert.ok(failAudit);
    assert.equal((failAudit?.detail as { error_code: string }).error_code, 'missing_header');
  });

  it('auth-failed requests do NOT parse the body, NOT transition state, NOT update memory (load-bearing fail-closed)', async () => {
    const h = buildHarness();
    let parserCalls = 0;
    h.deps.replyParser.parse = async () => {
      parserCalls++;
      // If the route somehow reached the parser, return a STOP intent
      // — the test below proves that even with this "would have
      // flipped stop_active" response, NOTHING is actually written
      // because auth failed before parser invocation.
      return Object.freeze({
        ok: true as const,
        source: 'deterministic' as const,
        intent: 'stop' as const
      });
    };
    const body = inboundBody({ content: 'STOP' });
    const r = await handleSendBlueInbound(
      { body, secretHeaderValue: 'wrong-secret-value-must-be-rejected' },
      h.deps
    );
    assert.equal(r.status, 401);
    // Parser was NEVER called.
    assert.equal(parserCalls, 0);
    // stop_active was NEVER written.
    assert.equal(await h.memoryStore.get(FOUNDER_USER, 'stop_active', null), null);
    // No feedback events written.
    assert.equal(await h.feedbackStore.countByKind(FOUNDER_USER, 'stop'), 0);
    // No inbound_replies row.
    assert.equal(await h.inboundReplyStore.count(FOUNDER_USER), 0);
  });
});

/* ====================================================================== */
/* Payload validation                                                     */
/* ====================================================================== */

describe('handleSendBlueInbound — payload validation', () => {
  it('returns 400 on non-JSON body', async () => {
    const h = buildHarness();
    const body = 'not json';
    const r = await handleSendBlueInbound(
      { body, secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );
    assert.equal(r.status, 400);
    const audits = await h.auditStore.recent(null as unknown as string, 50);
    assert.ok(
      audits.find(
        (e) =>
          e.action === 'fomo.sendblue.payload_invalid' &&
          (e.detail as { error_code?: string })?.error_code === 'body_not_json'
      )
    );
  });

  it('returns 400 on missing required fields', async () => {
    const h = buildHarness();
    const body = JSON.stringify({ random: 'payload' });
    const r = await handleSendBlueInbound(
      { body, secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );
    assert.equal(r.status, 400);
    const audits = await h.auditStore.recent(null as unknown as string, 50);
    assert.ok(
      audits.find(
        (e) =>
          e.action === 'fomo.sendblue.payload_invalid' &&
          (e.detail as { error_code?: string })?.error_code === 'missing_required_fields'
      )
    );
  });
});

/* ====================================================================== */
/* From-number allowlist (defense-in-depth)                               */
/* ====================================================================== */

describe('handleSendBlueInbound — from-number allowlist', () => {
  it('returns 403 + audits reply_unauthorized when from-number is not the founder', async () => {
    const h = buildHarness();
    const body = inboundBody({ from_number: '+19998887777' });
    const r = await handleSendBlueInbound(
      { body, secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );
    assert.equal(r.status, 403);
    const audits = await h.auditStore.recent(null as unknown as string, 50);
    const unauthorized = audits.find((e) => e.action === 'fomo.sendblue.reply_unauthorized');
    assert.ok(unauthorized);
    // LOAD-BEARING: must NEVER persist the full from-phone, only the slug suffix.
    const detail = unauthorized!.detail as { from_slug: string };
    assert.equal(detail.from_slug, '7777');
    // And the full number must not appear anywhere in the detail.
    assert.ok(!JSON.stringify(unauthorized!.detail).includes('+19998887777'));
  });
});

/* ====================================================================== */
/* Idempotency (LOAD-BEARING — SendBlue retry safety)                     */
/* ====================================================================== */

describe('handleSendBlueInbound — idempotency on SendBlue retry', () => {
  it('first request inserts inbound_replies; second request with same provider_message_id returns 200 + audits reply_duplicate + no double-processing', async () => {
    const h = buildHarness();
    // Use a stub parser that records call count.
    let parserCalls = 0;
    h.deps.replyParser.parse = async () => {
      parserCalls++;
      return Object.freeze({
        ok: true as const,
        source: 'deterministic' as const,
        intent: 'stop' as const
      });
    };
    const body = inboundBody({ content: 'STOP', message_handle: 'sb-msg-dup' });

    const r1 = await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(r1.status, 200);
    const r2 = await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(r2.status, 200);

    // Parser was called exactly ONCE (the second request short-circuited
    // at the inbound_replies UNIQUE gate before reaching the parser).
    assert.equal(parserCalls, 1);

    // Dedup store has exactly one row.
    assert.equal(await h.inboundReplyStore.count(FOUNDER_USER), 1);

    // feedback_events.stop count is exactly 1 (not doubled).
    assert.equal(await h.feedbackStore.countByKind(FOUNDER_USER, 'stop'), 1);

    // Audit shows exactly one stop_recorded + one reply_duplicate.
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    assert.equal(audits.filter((e) => e.action === 'fomo.sendblue.stop_recorded').length, 1);
    assert.equal(audits.filter((e) => e.action === 'fomo.sendblue.reply_duplicate').length, 1);
  });
});

/* ====================================================================== */
/* Deterministic STOP — full chain                                        */
/* ====================================================================== */

describe('handleSendBlueInbound — STOP (deterministic compliance)', () => {
  it('STOP writes stop feedback + stop_active memory_signal + audits stop_recorded WITHOUT touching the LLM', async () => {
    const h = buildHarness();
    let parserCalls = 0;
    h.deps.replyParser.parse = async () => {
      parserCalls++;
      // Real orchestrator's deterministic path would not reach the
      // mock router — but to simulate the route correctly, we route
      // through the real parseReply contract: the route hands it the
      // raw reply, and deterministic pre-pass returns immediately.
      // The stub returns deterministic-stop to mimic the orchestrator.
      return Object.freeze({
        ok: true as const,
        source: 'deterministic' as const,
        intent: 'stop' as const
      });
    };
    const body = inboundBody({ content: 'STOP' });
    const r = await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(r.status, 200);
    // Parser was called once (route hands the raw text to the
    // parser, which short-circuits deterministically — caller does
    // not know it's deterministic vs classifier from the contract
    // perspective).
    assert.equal(parserCalls, 1);

    // stop_active memory signal is upserted with active=true.
    const sig = await h.memoryStore.get(FOUNDER_USER, 'stop_active', null);
    assert.ok(sig);
    assert.equal((sig?.detail as { active: boolean }).active, true);
    assert.equal(sig?.source, 'user_confirmed');

    // feedback event of kind 'stop' is written.
    assert.equal(await h.feedbackStore.countByKind(FOUNDER_USER, 'stop'), 1);

    // Audit row fomo.sendblue.stop_recorded with stop_active: true.
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const stopAudit = audits.find((e) => e.action === 'fomo.sendblue.stop_recorded');
    assert.ok(stopAudit);
    assert.equal((stopAudit?.detail as { stop_active: boolean }).stop_active, true);
  });

  it('START flips stop_active back to false', async () => {
    const h = buildHarness();
    // First, set stop_active=true.
    await h.memoryStore.upsert({
      user_id: FOUNDER_USER,
      kind: 'stop_active',
      scope_key: null,
      detail: { active: true, recorded_at: new Date().toISOString() },
      source: 'user_confirmed',
      confidence: 1.0
    });
    h.deps.replyParser.parse = async () =>
      Object.freeze({
        ok: true as const,
        source: 'deterministic' as const,
        intent: 'start' as const
      });
    const body = inboundBody({ content: 'START' });
    const r = await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(r.status, 200);
    const sig = await h.memoryStore.get(FOUNDER_USER, 'stop_active', null);
    assert.equal((sig?.detail as { active: boolean }).active, false);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    assert.ok(audits.find((e) => e.action === 'fomo.sendblue.start_recorded'));
  });
});

/* ====================================================================== */
/* Classifier intent — snooze (with state-machine transitions)            */
/* ====================================================================== */

describe('handleSendBlueInbound — snooze (classifier, with matched alert)', () => {
  it('transitions alert sent → replied → snoozed, writes user_snoozed feedback with snooze_until', async () => {
    const h = buildHarness();
    await seedSentAlert(h, 'alert-snooze-1');
    h.deps.replyParser.parse = async () =>
      Object.freeze({
        ok: true as const,
        source: 'classifier' as const,
        classification: Object.freeze({
          intent: 'snooze' as const,
          confidence: 0.95,
          reason: 'tomorrow',
          snooze_hint: 'tomorrow' as const
        }),
        model_name: 'mock',
        prompt_version: 'reply-parser-v0.1.0',
        latency_ms: 10,
        input_tokens: 5,
        output_tokens: 5,
        estimated_cost_usd: 0,
        low_confidence_forced_unclear: false
      });
    const body = inboundBody({ content: 'tomorrow' });
    const r = await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(r.status, 200);
    // State machine walked all the way to snoozed.
    assert.equal(await h.transitions.currentState('alert-snooze-1'), 'snoozed');
    // Phase v0.5.10 — feedback_event is now written by the routing
    // module with the GENERIC verb 'snoozed' (not the legacy 'user_snoozed'
    // kind). The detail.dimension='alert' overlay marks it as a per-alert
    // snooze. snooze_hint is forwarded.
    const fb = await h.feedbackStore.recent(FOUNDER_USER, 20);
    const snoozeFb = fb.find((e) => e.kind === 'snoozed');
    assert.ok(snoozeFb);
    assert.equal(snoozeFb?.alert_id, 'alert-snooze-1');
    const detail = snoozeFb?.detail as { dimension: string; role: string; snooze_hint: string };
    assert.equal(detail.dimension, 'alert');
    assert.equal(detail.role, 'user');
    assert.equal(detail.snooze_hint, 'tomorrow');
  });
});

/* ====================================================================== */
/* Classifier intent — ignore_sender                                      */
/* ====================================================================== */

describe('handleSendBlueInbound — ignore_sender (sender-suppression memory signal)', () => {
  it('writes sender_suppressed memory signal + ignored_sender feedback + transitions sent → replied → ignored', async () => {
    const h = buildHarness();
    await seedSentAlert(h, 'alert-suppress-1');
    h.deps.replyParser.parse = async () =>
      Object.freeze({
        ok: true as const,
        source: 'classifier' as const,
        classification: Object.freeze({
          intent: 'ignore_sender' as const,
          confidence: 0.9,
          reason: 'sender suppression',
          snooze_hint: null
        }),
        model_name: 'mock',
        prompt_version: 'reply-parser-v0.1.0',
        latency_ms: 10,
        input_tokens: 5,
        output_tokens: 5,
        estimated_cost_usd: 0,
        low_confidence_forced_unclear: false
      });
    const body = inboundBody({ content: 'never alert me about this sender again' });
    const r = await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(r.status, 200);
    assert.equal(await h.transitions.currentState('alert-suppress-1'), 'ignored');
    // Phase v0.5.10 — feedback_event is now written by the routing
    // module with the GENERIC verb 'ignored' + detail.dimension='sender'
    // (not the legacy 'ignored_sender' kind). The existing v0.5.5
    // sender_suppressed memory_signal write is unchanged (still owned by
    // applyIgnoreSender, separate from the v0.5.9 substrate).
    assert.equal(await h.feedbackStore.countByKind(FOUNDER_USER, 'ignored'), 1);
    const sig = await h.memoryStore.get(
      FOUNDER_USER,
      'sender_suppressed',
      `message:msg-alert-suppress-1`
    );
    assert.ok(sig);
  });
});

/* ====================================================================== */
/* Classifier intent — unclear (no state transition past replied)         */
/* ====================================================================== */

describe('handleSendBlueInbound — unclear (fail-safe; no state transition)', () => {
  it('audits reply_unclear; does NOT transition alert state', async () => {
    const h = buildHarness();
    await seedSentAlert(h, 'alert-unclear-1');
    h.deps.replyParser.parse = async () =>
      Object.freeze({
        ok: true as const,
        source: 'classifier' as const,
        classification: Object.freeze({
          intent: 'unclear' as const,
          confidence: 0.2,
          reason: 'gibberish',
          snooze_hint: null
        }),
        model_name: 'mock',
        prompt_version: 'reply-parser-v0.1.0',
        latency_ms: 10,
        input_tokens: 5,
        output_tokens: 5,
        estimated_cost_usd: 0,
        low_confidence_forced_unclear: true
      });
    const body = inboundBody({ content: 'asdf' });
    const r = await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    assert.equal(r.status, 200);
    // Alert stayed in 'sent' (no transition past replied for unclear).
    assert.equal(await h.transitions.currentState('alert-unclear-1'), 'sent');
    // No feedback event written.
    assert.equal(await h.feedbackStore.recent(FOUNDER_USER, 50).then((rows) => rows.length), 0);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    assert.ok(audits.find((e) => e.action === 'fomo.sendblue.reply_unclear'));
  });
});

/* ====================================================================== */
/* Memory V1 visible memory command adapter seam (disabled by default)     */
/* ====================================================================== */

describe('handleSendBlueInbound — disabled visible memory command adapter seam', () => {
  class CountingTypedMemoryStore extends InMemoryTypedMemoryStore {
    listCount = 0;
    writeCount = 0;
    retractCount = 0;

    override async listActive(...args: Parameters<InMemoryTypedMemoryStore['listActive']>) {
      this.listCount += 1;
      return super.listActive(...args);
    }

    override async write(input: NewTypedMemoryRow) {
      this.writeCount += 1;
      return super.write(input);
    }

    override async retract(
      userId: string,
      kind: Parameters<InMemoryTypedMemoryStore['retract']>[1],
      scopeKey: string,
      supersededBy: number | null = null
    ) {
      this.retractCount += 1;
      return super.retract(userId, kind, scopeKey, supersededBy);
    }
  }

  it('is inert when absent or disabled and preserves the existing SendBlue reply path', async () => {
    const store = new CountingTypedMemoryStore();
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: false,
        store,
        context: {
          text: 'Remember this: I prefer mornings',
          parsedPreference: {
            attribute: 'alert_timing',
            value: 'alice@example.com mornings only',
            sourceRef: 'reply:private-disabled-sendblue-memory-ref'
          }
        }
      },
      parser: async () =>
        Object.freeze({
          ok: true as const,
          source: 'classifier' as const,
          classification: Object.freeze({
            intent: 'snooze' as const,
            confidence: 0.9,
            reason: 'snooze',
            snooze_hint: 'tomorrow' as const
          }),
          model_name: 'mock',
          prompt_version: 'reply-parser-v0.1.0',
          latency_ms: 1,
          input_tokens: 1,
          output_tokens: 1,
          estimated_cost_usd: 0,
          low_confidence_forced_unclear: false
        })
    });
    await seedSentAlert(h, 'alert-memory-disabled-1');

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: 'tomorrow' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.equal(await h.transitions.currentState('alert-memory-disabled-1'), 'snoozed');
    assert.equal(store.listCount, 0);
    assert.equal(store.writeCount, 0);
    assert.equal(store.retractCount, 0);
    assert.equal(
      (await h.auditStore.recent(FOUNDER_USER, 50)).some(
        (event) => event.action === 'visible_memory_command.app_adapter.outcome'
      ),
      false
    );
  });

  it('when enabled, calls the app adapter only with caller-supplied memory context, not inbound freeform text', async () => {
    const store = new CountingTypedMemoryStore();
    const INBOUND_PRIVATE_TEXT = 'freeform inbound text alice@example.com must not become memory';
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        context: {
          text: 'Remember this: I prefer mornings',
          parsedPreference: {
            attribute: 'alert_timing',
            value: 'mornings',
            updatedAt: '2026-07-09T12:00:00.000Z',
            sourceRef: 'reply:private-sendblue-memory-command-ref'
          }
        }
      }
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: INBOUND_PRIVATE_TEXT }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    const recall = await recallVisibleExplicitPreference(store, FOUNDER_USER, {
      attribute: 'alert_timing'
    });
    assert.equal(recall?.preference_summary, 'alert timing: mornings');
    assert.equal(store.writeCount, 1);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const memoryAudit = audits.find(
      (event) => event.action === 'visible_memory_command.app_adapter.outcome'
    );
    assert.ok(memoryAudit);
    assert.equal((memoryAudit.detail as { status?: string }).status, 'handled');
    const auditJson = JSON.stringify(memoryAudit.detail);
    assert.equal(auditJson.includes(INBOUND_PRIVATE_TEXT), false);
    assert.equal(auditJson.includes('alice@example.com'), false);
    assert.equal(auditJson.includes('mornings'), false);
    assert.equal(auditJson.includes('private-sendblue-memory-command-ref'), false);
  });

  it('is inert when enabled without direct context or resolver context', async () => {
    const store = new CountingTypedMemoryStore();
    const capturedResponses: SendBlueInboundVisibleMemoryCommandResponseEnvelope[] = [];
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        response: {
          enabled: true,
          record: (envelope) => {
            capturedResponses.push(envelope);
          }
        }
      }
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: 'Remember this: I prefer evenings' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.equal(store.writeCount, 0);
    assert.equal(store.listCount, 0);
    assert.equal(store.retractCount, 0);
    assert.deepEqual(capturedResponses, []);
    assert.deepEqual(JSON.parse(r.body), { ok: true, intent: 'unclear', source: 'classifier' });
    assert.equal(
      (await h.auditStore.recent(FOUNDER_USER, 50)).some(
        (event) => event.action === 'visible_memory_command.app_adapter.outcome'
      ),
      false
    );
  });

  it('captures a sanitized visible-memory app-adapter response only when the response seam is enabled', async () => {
    const store = new CountingTypedMemoryStore();
    await store.write({
      user_id: FOUNDER_USER,
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'founder private mornings alice@example.com',
      source: 'user_stated',
      source_ref: 'reply:founder-private-response-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T11:00:00.000Z'
    } as NewTypedMemoryRow);
    await store.write({
      user_id: 'other-user',
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'other-user private evenings alice@example.com',
      source: 'user_stated',
      source_ref: 'reply:other-user-private-response-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T12:00:00.000Z'
    } as NewTypedMemoryRow);
    store.writeCount = 0;
    const capturedResponses: SendBlueInboundVisibleMemoryCommandResponseEnvelope[] = [];
    const INBOUND_PRIVATE_TEXT = 'raw inbound asks memory alice@example.com';
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        context: {
          text: 'What do you remember about me?',
          query: { attribute: 'alert_timing' }
        },
        response: {
          enabled: true,
          record: (envelope) => {
            capturedResponses.push(envelope);
          }
        }
      }
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: INBOUND_PRIVATE_TEXT }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.deepEqual(JSON.parse(r.body), { ok: true, intent: 'unclear', source: 'classifier' });
    assert.equal(capturedResponses.length, 1);
    assert.equal(capturedResponses[0]!.handled, true);
    assert.equal(capturedResponses[0]!.status, 'handled');
    assert.equal(capturedResponses[0]!.user_id, FOUNDER_USER);
    assert.equal(capturedResponses[0]!.audit_metadata.matched_intent, 'memory_review');
    assert.equal(capturedResponses[0]!.audit_metadata.returned_count, 1);
    assert.equal(capturedResponses[0]!.response.text?.includes('[redacted]'), true);
    assert.equal('command_result' in capturedResponses[0]!, false);
    const responseJson = JSON.stringify(capturedResponses);
    assert.equal(responseJson.includes(INBOUND_PRIVATE_TEXT), false);
    assert.equal(responseJson.includes('alice@example.com'), false);
    assert.equal(responseJson.includes('founder private mornings'), false);
    assert.equal(responseJson.includes('founder-private-response-ref'), false);
    assert.equal(responseJson.includes('other-user'), false);
    assert.equal(responseJson.includes('other-user-private-response-ref'), false);
  });

  it('does not capture a response when the response seam is disabled', async () => {
    const store = new CountingTypedMemoryStore();
    const capturedResponses: SendBlueInboundVisibleMemoryCommandResponseEnvelope[] = [];
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        context: {
          text: 'Remember this: I prefer mornings',
          parsedPreference: { attribute: 'alert_timing', value: 'mornings' }
        },
        response: {
          enabled: false,
          record: (envelope) => {
            capturedResponses.push(envelope);
          }
        }
      }
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: 'memory command' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.equal(store.writeCount, 1);
    assert.deepEqual(capturedResponses, []);
    assert.deepEqual(JSON.parse(r.body), { ok: true, intent: 'unclear', source: 'classifier' });
  });

  it('resolver supplies only caller-provided parsed context and never receives inbound freeform text', async () => {
    const store = new CountingTypedMemoryStore();
    const resolverInputs: unknown[] = [];
    const INBOUND_PRIVATE_TEXT = 'raw inbound says remember alice@example.com as the secret value';
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextResolver: (input): SendBlueInboundVisibleMemoryCommandContext => {
          resolverInputs.push(input);
          return {
            text: 'Remember this: I prefer afternoon alerts',
            parsedPreference: {
              attribute: 'alert_timing',
              value: 'afternoons',
              sourceRef: 'reply:private-resolver-ref'
            }
          };
        }
      },
      parser: async () =>
        Object.freeze({
          ok: true as const,
          source: 'classifier' as const,
          classification: Object.freeze({
            intent: 'unclear' as const,
            confidence: 0.3,
            reason: 'unclear',
            snooze_hint: null
          }),
          model_name: 'mock',
          prompt_version: 'reply-parser-v0.1.0',
          latency_ms: 1,
          input_tokens: 1,
          output_tokens: 1,
          estimated_cost_usd: 0,
          low_confidence_forced_unclear: false
        })
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: INBOUND_PRIVATE_TEXT }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.equal(store.writeCount, 1);
    const resolverInput = resolverInputs[0] as { userId?: unknown; parsedIntent?: unknown; now?: unknown };
    assert.equal(resolverInputs.length, 1);
    assert.equal(resolverInput.userId, FOUNDER_USER);
    assert.equal(resolverInput.parsedIntent, 'unclear');
    assert.ok(resolverInput.now instanceof Date);
    const resolverInputJson = JSON.stringify(resolverInputs);
    assert.equal(resolverInputJson.includes(INBOUND_PRIVATE_TEXT), false);
    assert.equal(resolverInputJson.includes('alice@example.com'), false);

    const recall = await recallVisibleExplicitPreference(store, FOUNDER_USER, {
      attribute: 'alert_timing'
    });
    assert.equal(recall?.preference_summary, 'alert timing: afternoons');
    const auditJson = JSON.stringify(await h.auditStore.recent(FOUNDER_USER, 50));
    assert.equal(auditJson.includes(INBOUND_PRIVATE_TEXT), false);
    assert.equal(auditJson.includes('alice@example.com'), false);
    assert.equal(auditJson.includes('afternoons'), false);
    assert.equal(auditJson.includes('private-resolver-ref'), false);
  });

  it('resolver can supply caller-provided query context without leaking cross-user/private memory', async () => {
    const store = new CountingTypedMemoryStore();
    await store.write({
      user_id: FOUNDER_USER,
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'founder private mornings alice@example.com',
      source: 'user_stated',
      source_ref: 'reply:founder-private-resolver-query-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T11:00:00.000Z'
    } as NewTypedMemoryRow);
    await store.write({
      user_id: 'other-user',
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'other-user private evenings alice@example.com',
      source: 'user_stated',
      source_ref: 'reply:other-user-private-resolver-query-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T12:00:00.000Z'
    } as NewTypedMemoryRow);
    store.writeCount = 0;
    const INBOUND_PRIVATE_TEXT = 'raw inbound asks for memory alice@example.com';
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextResolver: (): SendBlueInboundVisibleMemoryCommandContext => ({
          text: 'What do you remember about me?',
          query: { attribute: 'alert_timing' }
        })
      }
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: INBOUND_PRIVATE_TEXT }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.equal(store.writeCount, 0);
    assert.equal(store.listCount, 1);
    const memoryAudit = (await h.auditStore.recent(FOUNDER_USER, 50)).find(
      (event) => event.action === 'visible_memory_command.app_adapter.outcome'
    );
    assert.ok(memoryAudit);
    assert.equal((memoryAudit.detail as { returned_count?: number }).returned_count, 1);
    const auditJson = JSON.stringify(await h.auditStore.recent(FOUNDER_USER, 50));
    assert.equal(auditJson.includes(INBOUND_PRIVATE_TEXT), false);
    assert.equal(auditJson.includes('alice@example.com'), false);
    assert.equal(auditJson.includes('founder-private-resolver-query-ref'), false);
    assert.equal(auditJson.includes('other-user'), false);
    assert.equal(auditJson.includes('other-user-private-resolver-query-ref'), false);
  });

  it('resolver can supply caller-provided correction context without deriving values from inbound text', async () => {
    const store = new CountingTypedMemoryStore();
    await store.write({
      user_id: FOUNDER_USER,
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'evenings',
      source: 'user_stated',
      source_ref: 'reply:private-old-resolver-correction-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T11:00:00.000Z'
    } as NewTypedMemoryRow);
    store.writeCount = 0;
    const INBOUND_PRIVATE_TEXT = 'raw inbound says correct alice@example.com to secret late nights';
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextResolver: (): SendBlueInboundVisibleMemoryCommandContext => ({
          text: 'Correct that preference',
          query: { attribute: 'alert_timing' },
          correction: {
            correctedValue: 'mornings',
            sourceRef: 'reply:private-new-resolver-correction-ref',
            updatedAt: '2026-07-09T13:00:00.000Z'
          }
        })
      }
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: INBOUND_PRIVATE_TEXT }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.equal(store.writeCount, 1);
    const recall = await recallVisibleExplicitPreference(store, FOUNDER_USER, {
      attribute: 'alert_timing'
    });
    assert.equal(recall?.preference_summary, 'alert timing: mornings');
    const auditJson = JSON.stringify(await h.auditStore.recent(FOUNDER_USER, 50));
    assert.equal(auditJson.includes(INBOUND_PRIVATE_TEXT), false);
    assert.equal(auditJson.includes('alice@example.com'), false);
    assert.equal(auditJson.includes('late nights'), false);
    assert.equal(auditJson.includes('mornings'), false);
    assert.equal(auditJson.includes('private-old-resolver-correction-ref'), false);
    assert.equal(auditJson.includes('private-new-resolver-correction-ref'), false);
  });

  it('does not invoke memory command handling for STOP even when enabled context is supplied', async () => {
    const store = new CountingTypedMemoryStore();
    const capturedResponses: SendBlueInboundVisibleMemoryCommandResponseEnvelope[] = [];
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        context: {
          text: 'Remember this: I prefer mornings',
          parsedPreference: { attribute: 'alert_timing', value: 'mornings' }
        },
        response: {
          enabled: true,
          record: (envelope) => {
            capturedResponses.push(envelope);
          }
        }
      },
      parser: async () => Object.freeze({ ok: true as const, source: 'deterministic' as const, intent: 'stop' as const })
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: 'STOP' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.equal((await h.memoryStore.get(FOUNDER_USER, 'stop_active', null))?.detail.active, true);
    assert.equal(store.writeCount, 0);
    assert.equal(store.listCount, 0);
    assert.deepEqual(capturedResponses, []);
    assert.equal(
      (await h.auditStore.recent(FOUNDER_USER, 50)).some(
        (event) => event.action === 'visible_memory_command.app_adapter.outcome'
      ),
      false
    );
  });

  it('does not resolve memory command context for STOP', async () => {
    const store = new CountingTypedMemoryStore();
    let resolverCount = 0;
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextResolver: () => {
          resolverCount += 1;
          return {
            text: 'Remember this: I prefer mornings',
            parsedPreference: { attribute: 'alert_timing', value: 'mornings' }
          };
        }
      },
      parser: async () => Object.freeze({ ok: true as const, source: 'deterministic' as const, intent: 'stop' as const })
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: 'STOP' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.equal(resolverCount, 0);
    assert.equal(store.writeCount, 0);
    assert.equal(store.listCount, 0);
  });

  it('does not resolve memory command context for START', async () => {
    const store = new CountingTypedMemoryStore();
    let resolverCount = 0;
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextResolver: () => {
          resolverCount += 1;
          return {
            text: 'Remember this: I prefer mornings',
            parsedPreference: { attribute: 'alert_timing', value: 'mornings' }
          };
        }
      },
      parser: async () => Object.freeze({ ok: true as const, source: 'deterministic' as const, intent: 'start' as const })
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: 'START' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.equal(resolverCount, 0);
    assert.equal(store.writeCount, 0);
    assert.equal(store.listCount, 0);
    assert.equal((await h.memoryStore.get(FOUNDER_USER, 'stop_active', null))?.detail.active, false);
  });

  it('keeps enabled route-level review scoped to the resolved user and sanitized audit metadata', async () => {
    const store = new CountingTypedMemoryStore();
    await store.write({
      user_id: 'other-user',
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'other-user secret alice@example.com',
      source: 'user_stated',
      source_ref: 'reply:other-user-secret-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T11:00:00.000Z'
    } as NewTypedMemoryRow);
    store.writeCount = 0;
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        context: {
          text: 'What do you remember about me?',
          query: { attribute: 'alert_timing' }
        }
      }
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: 'memory review please' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.equal(store.writeCount, 0);
    const memoryAudit = (await h.auditStore.recent(FOUNDER_USER, 50)).find(
      (event) => event.action === 'visible_memory_command.app_adapter.outcome'
    );
    assert.ok(memoryAudit);
    assert.equal((memoryAudit.detail as { returned_count?: number }).returned_count, 0);
    const allAuditJson = JSON.stringify(await h.auditStore.recent(FOUNDER_USER, 50));
    assert.equal(allAuditJson.includes('other-user'), false);
    assert.equal(allAuditJson.includes('alice@example.com'), false);
    assert.equal(allAuditJson.includes('other-user-secret-ref'), false);
  });
  it('exact-command parser helper accepts only explicit visible-memory command forms', () => {
    const baseInput = Object.freeze({
      userId: FOUNDER_USER,
      parsedIntent: 'unclear' as const,
      now: new Date('2026-07-10T12:00:00.000Z')
    });

    assert.deepEqual(
      parseSendBlueInboundVisibleMemoryExactCommandContext({
        ...baseInput,
        text: 'remember preference alert_timing: mornings only'
      }),
      {
        text: 'Remember this preference',
        parsedPreference: {
          attribute: 'alert_timing',
          value: 'mornings only',
          sourceRef: 'reply:sendblue-visible-memory-exact-command',
          source: 'user_stated',
          confidence: 'high',
          updatedAt: '2026-07-10T12:00:00.000Z'
        }
      }
    );
    assert.deepEqual(
      parseSendBlueInboundVisibleMemoryExactCommandContext({ ...baseInput, text: 'review memory alert_timing' }),
      { text: 'What do you remember about me?', query: { attribute: 'alert_timing' } }
    );
    assert.deepEqual(
      parseSendBlueInboundVisibleMemoryExactCommandContext({ ...baseInput, text: 'explain memory alert_timing' }),
      { text: 'Why did you remember that?', query: { attribute: 'alert_timing' } }
    );
    assert.deepEqual(
      parseSendBlueInboundVisibleMemoryExactCommandContext({ ...baseInput, text: 'forget memory alert_timing' }),
      { text: 'Forget that preference', query: { attribute: 'alert_timing' } }
    );
    assert.deepEqual(
      parseSendBlueInboundVisibleMemoryExactCommandContext({
        ...baseInput,
        text: 'correct memory alert_timing: afternoons'
      }),
      {
        text: 'Correct that preference',
        query: { attribute: 'alert_timing' },
        correction: {
          correctedValue: 'afternoons',
          sourceRef: 'reply:sendblue-visible-memory-exact-command',
          source: 'user_stated',
          updatedAt: '2026-07-10T12:00:00.000Z'
        }
      }
    );

    for (const text of [
      'I like mornings',
      'please remember that I like mornings',
      'remember preference alert_timing',
      'remember preference alice@example.com: mornings',
      'correct memory alert_timing',
      'why did you remember that?',
      'STOP',
      'START'
    ]) {
      assert.equal(
        parseSendBlueInboundVisibleMemoryExactCommandContext({ ...baseInput, text }),
        null,
        `expected non-exact text to be ignored: ${text}`
      );
    }
  });

  it('is inert when the exact-command context parser is absent or disabled', async () => {
    const store = new CountingTypedMemoryStore();
    const capturedResponses: SendBlueInboundVisibleMemoryCommandResponseEnvelope[] = [];
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextParser: { enabled: false },
        response: {
          enabled: true,
          record: (envelope) => {
            capturedResponses.push(envelope);
          }
        }
      }
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: 'remember preference alert_timing: mornings' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.deepEqual(JSON.parse(r.body), { ok: true, intent: 'unclear', source: 'classifier' });
    assert.equal(store.writeCount, 0);
    assert.equal(store.listCount, 0);
    assert.equal(store.retractCount, 0);
    assert.deepEqual(capturedResponses, []);
    assert.equal(
      (await h.auditStore.recent(FOUNDER_USER, 50)).some(
        (event) => event.action === 'visible_memory_command.app_adapter.outcome'
      ),
      false
    );
  });

  it('when test-enabled, parses exact remember commands without changing the public HTTP response shape or leaking private values', async () => {
    const store = new CountingTypedMemoryStore();
    const capturedResponses: SendBlueInboundVisibleMemoryCommandResponseEnvelope[] = [];
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextParser: { enabled: true },
        response: {
          enabled: true,
          record: (envelope) => {
            capturedResponses.push(envelope);
          }
        }
      }
    });
    const INBOUND_PRIVATE_COMMAND = 'remember preference alert_timing: mornings alice@example.com';

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: INBOUND_PRIVATE_COMMAND }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.deepEqual(JSON.parse(r.body), { ok: true, intent: 'unclear', source: 'classifier' });
    assert.equal(store.writeCount, 1);
    assert.equal(capturedResponses.length, 1);
    assert.equal(capturedResponses[0]!.handled, true);
    assert.equal(capturedResponses[0]!.audit_metadata.matched_intent, 'memory_remember');
    assert.equal(capturedResponses[0]!.response.text?.includes('mornings'), false);
    const auditJson = JSON.stringify(await h.auditStore.recent(FOUNDER_USER, 50));
    const responseJson = JSON.stringify(capturedResponses);
    assert.equal(auditJson.includes(INBOUND_PRIVATE_COMMAND), false);
    assert.equal(auditJson.includes('alice@example.com'), false);
    assert.equal(auditJson.includes('mornings'), false);
    assert.equal(auditJson.includes('sendblue-visible-memory-exact-command'), false);
    assert.equal(responseJson.includes(INBOUND_PRIVATE_COMMAND), false);
    assert.equal(responseJson.includes('alice@example.com'), false);
    assert.equal(responseJson.includes('mornings'), false);
    assert.equal(responseJson.includes('sendblue-visible-memory-exact-command'), false);
  });

  it('when test-enabled, parses exact review/explain/forget/correct commands with user scoping and no cross-user leakage', async () => {
    const store = new CountingTypedMemoryStore();
    await store.write({
      user_id: FOUNDER_USER,
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'founder private mornings alice@example.com',
      source: 'user_stated',
      source_ref: 'reply:founder-private-exact-parser-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T11:00:00.000Z'
    } as NewTypedMemoryRow);
    await store.write({
      user_id: 'other-user',
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'other-user private evenings alice@example.com',
      source: 'user_stated',
      source_ref: 'reply:other-user-private-exact-parser-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T12:00:00.000Z'
    } as NewTypedMemoryRow);
    store.writeCount = 0;
    const capturedResponses: SendBlueInboundVisibleMemoryCommandResponseEnvelope[] = [];
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextParser: { enabled: true },
        response: {
          enabled: true,
          record: (envelope) => {
            capturedResponses.push(envelope);
          }
        }
      }
    });

    for (const content of [
      'review memory alert_timing',
      'explain memory alert_timing',
      'correct memory alert_timing: afternoons',
      'forget memory alert_timing'
    ]) {
      const r = await handleSendBlueInbound(
        { body: inboundBody({ content, message_handle: `sb-${content.split(' ')[0]}` }), secretHeaderValue: WEBHOOK_SECRET },
        h.deps
      );
      assert.equal(r.status, 200);
      assert.deepEqual(JSON.parse(r.body), { ok: true, intent: 'unclear', source: 'classifier' });
    }

    assert.equal(capturedResponses.length, 4);
    assert.deepEqual(
      capturedResponses.map((response) => response.audit_metadata.matched_intent),
      ['memory_review', 'memory_explanation', 'memory_correct', 'memory_forget']
    );
    const auditJson = JSON.stringify(await h.auditStore.recent(FOUNDER_USER, 100));
    const responseJson = JSON.stringify(capturedResponses);
    for (const forbidden of [
      'alice@example.com',
      'founder private mornings',
      'founder-private-exact-parser-ref',
      'other-user',
      'other-user private evenings',
      'other-user-private-exact-parser-ref',
      'afternoons',
      'sendblue-visible-memory-exact-command'
    ]) {
      assert.equal(auditJson.includes(forbidden), false, `audit leaked ${forbidden}`);
      assert.equal(responseJson.includes(forbidden), false, `response leaked ${forbidden}`);
    }
  });

  it('does not parse STOP or START through the exact-command context parser', async () => {
    for (const command of ['STOP', 'START'] as const) {
      const store = new CountingTypedMemoryStore();
      let parserCount = 0;
      const h = buildHarness({
        visibleMemoryCommand: {
          enabled: true,
          store,
          contextParser: {
            enabled: true,
            parse: () => {
              parserCount += 1;
              return {
                text: 'Remember this preference',
                parsedPreference: { attribute: 'alert_timing', value: 'mornings' }
              };
            }
          }
        },
        parser: async () =>
          Object.freeze({
            ok: true as const,
            source: 'deterministic' as const,
            intent: command.toLowerCase() as 'stop' | 'start'
          })
      });

      const r = await handleSendBlueInbound(
        { body: inboundBody({ content: command }), secretHeaderValue: WEBHOOK_SECRET },
        h.deps
      );

      assert.equal(r.status, 200);
      assert.equal(parserCount, 0);
      assert.equal(store.writeCount, 0);
      assert.equal(store.listCount, 0);
      assert.equal(
        (await h.memoryStore.get(FOUNDER_USER, 'stop_active', null))?.detail.active,
        command === 'STOP'
      );
    }
  });

  it('reply-text renderer helper uses only sanitized app-adapter response text and structural metadata', () => {
    const rendered = renderSendBlueInboundVisibleMemoryCommandReplyText({
      handled: true,
      status: 'handled',
      user_id: FOUNDER_USER,
      response: { text: '  Safe reply text from adapter.\nNo raw command payload.  ' },
      audit_metadata: {
        memory_kind: 'preference',
        adapter: 'visible_memory_command_app_adapter',
        enabled: true,
        handler_status: 'handled',
        matched_action: 'review_visible_explicit_preferences',
        matched_intent: 'memory_review',
        returned_count: 1,
        row_ids: [123],
        scope_keys: ['preference:alert_timing']
      }
    });

    assert.deepEqual(rendered, {
      text: 'Safe reply text from adapter. No raw command payload.',
      metadata: {
        renderer: 'sendblue_visible_memory_command_reply_text_renderer',
        memory_kind: 'preference',
        handled: true,
        status: 'handled',
        matched_intent: 'memory_review',
        returned_count: 1
      }
    });
    const renderedJson = JSON.stringify(rendered);
    assert.equal(renderedJson.includes('row_ids'), false);
    assert.equal(renderedJson.includes('scope_keys'), false);
    assert.equal(renderedJson.includes('preference:alert_timing'), false);

    assert.deepEqual(
      renderSendBlueInboundVisibleMemoryCommandReplyText({
        handled: false,
        status: 'no_memory_command',
        user_id: FOUNDER_USER,
        response: { text: 'must not render when unhandled' },
        audit_metadata: {
          memory_kind: 'preference',
          adapter: 'visible_memory_command_app_adapter',
          enabled: true,
          handler_status: 'no_memory_command'
        }
      }),
      {
        text: null,
        metadata: {
          renderer: 'sendblue_visible_memory_command_reply_text_renderer',
          memory_kind: 'preference',
          handled: false,
          status: 'no_memory_command'
        }
      }
    );
  });

  it('reply-text renderer seam is inert by default and does not change the public HTTP response shape', async () => {
    const store = new CountingTypedMemoryStore();
    const capturedReplies: SendBlueInboundVisibleMemoryCommandReplyText[] = [];
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextParser: { enabled: true },
        response: {
          enabled: true,
          record: () => undefined,
          replyText: {
            enabled: false,
            record: (reply) => {
              capturedReplies.push(reply);
            }
          }
        }
      }
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: 'remember preference alert_timing: mornings' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.deepEqual(JSON.parse(r.body), { ok: true, intent: 'unclear', source: 'classifier' });
    assert.deepEqual(capturedReplies, []);
    assert.equal(store.writeCount, 1);
  });

  it('when test-enabled, renders deterministic reply text for remember/review/explain/correct/forget without leaking private or cross-user data', async () => {
    const store = new CountingTypedMemoryStore();
    await store.write({
      user_id: FOUNDER_USER,
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'founder private mornings alice@example.com',
      source: 'user_stated',
      source_ref: 'reply:founder-private-renderer-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T11:00:00.000Z'
    } as NewTypedMemoryRow);
    await store.write({
      user_id: 'other-user',
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'other-user private evenings alice@example.com',
      source: 'user_stated',
      source_ref: 'reply:other-user-private-renderer-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T12:00:00.000Z'
    } as NewTypedMemoryRow);
    store.writeCount = 0;
    const capturedReplies: SendBlueInboundVisibleMemoryCommandReplyText[] = [];
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextParser: { enabled: true },
        response: {
          enabled: true,
          replyText: {
            enabled: true,
            record: (reply) => {
              capturedReplies.push(reply);
            }
          }
        }
      }
    });

    for (const content of [
      'remember preference digest_style: concise alice@example.com',
      'review memory alert_timing',
      'explain memory alert_timing',
      'correct memory alert_timing: afternoons',
      'forget memory alert_timing'
    ]) {
      const r = await handleSendBlueInbound(
        { body: inboundBody({ content, message_handle: `sb-render-${content.split(' ')[0]}` }), secretHeaderValue: WEBHOOK_SECRET },
        h.deps
      );
      assert.equal(r.status, 200);
      assert.deepEqual(JSON.parse(r.body), { ok: true, intent: 'unclear', source: 'classifier' });
    }

    assert.deepEqual(
      capturedReplies.map((reply) => reply.metadata.matched_intent),
      ['memory_remember', 'memory_review', 'memory_explanation', 'memory_correct', 'memory_forget']
    );
    assert.equal(capturedReplies.every((reply) => reply.text !== null), true);
    assert.equal(capturedReplies.some((reply) => reply.text?.includes('[redacted]')), true);
    const renderedJson = JSON.stringify(capturedReplies);
    for (const forbidden of [
      'founder private mornings',
      'other-user',
      'other-user private evenings',
      'founder-private-renderer-ref',
      'other-user-private-renderer-ref',
      'sendblue-visible-memory-exact-command',
      'concise alice@example.com',
      'afternoons'
    ]) {
      assert.equal(renderedJson.includes(forbidden), false, `reply renderer leaked ${forbidden}`);
    }
  });

  it('reply delivery-candidate helper uses sanitized reply text, structural metadata, and in-memory recipient only', () => {
    const candidate = createSendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate({
      to: FOUNDER_PHONE,
      reply: {
        text: '  Safe reply from renderer.\n',
        metadata: {
          renderer: 'sendblue_visible_memory_command_reply_text_renderer',
          memory_kind: 'preference',
          handled: true,
          status: 'handled',
          matched_intent: 'memory_review',
          returned_count: 1
        }
      }
    });

    assert.deepEqual(candidate, {
      to: FOUNDER_PHONE,
      text: 'Safe reply from renderer.',
      metadata: {
        renderer: 'sendblue_visible_memory_command_reply_delivery_candidate',
        reply_text_renderer: 'sendblue_visible_memory_command_reply_text_renderer',
        memory_kind: 'preference',
        handled: true,
        status: 'handled',
        matched_intent: 'memory_review',
        returned_count: 1
      }
    });
    assert.equal(JSON.stringify(candidate?.metadata).includes(FOUNDER_PHONE), false);

    for (const reply of [
      {
        text: 'handled flag is false',
        metadata: {
          renderer: 'sendblue_visible_memory_command_reply_text_renderer' as const,
          memory_kind: 'preference' as const,
          handled: false,
          status: 'no_memory_command' as const
        }
      },
      {
        text: '   ',
        metadata: {
          renderer: 'sendblue_visible_memory_command_reply_text_renderer' as const,
          memory_kind: 'preference' as const,
          handled: true,
          status: 'handled' as const
        }
      }
    ]) {
      assert.equal(
        createSendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate({ to: FOUNDER_PHONE, reply }),
        null
      );
    }
  });

  it('reply delivery-candidate seam is inert by default and preserves HTTP/audit metadata', async () => {
    const store = new CountingTypedMemoryStore();
    const capturedCandidates: SendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate[] = [];
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextParser: { enabled: true },
        response: {
          enabled: true,
          replyText: {
            enabled: true,
            deliveryCandidate: {
              enabled: false,
              record: (candidate) => {
                capturedCandidates.push(candidate);
              }
            }
          }
        }
      }
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: 'remember preference alert_timing: mornings' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.deepEqual(JSON.parse(r.body), { ok: true, intent: 'unclear', source: 'classifier' });
    assert.deepEqual(capturedCandidates, []);
    const auditJson = JSON.stringify(await h.auditStore.recent(FOUNDER_USER, 50));
    assert.equal(auditJson.includes(FOUNDER_PHONE), false);
    assert.equal(auditJson.includes('mornings'), false);
  });

  it('when test-enabled, captures delivery candidates without exposing private values, source refs, inbound text, or full phone in audit/HTTP metadata', async () => {
    const store = new CountingTypedMemoryStore();
    await store.write({
      user_id: FOUNDER_USER,
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'founder private mornings alice@example.com',
      source: 'user_stated',
      source_ref: 'reply:founder-private-candidate-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T11:00:00.000Z'
    } as NewTypedMemoryRow);
    await store.write({
      user_id: 'other-user',
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'other-user private candidate evenings alice@example.com',
      source: 'user_stated',
      source_ref: 'reply:other-user-private-candidate-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T12:00:00.000Z'
    } as NewTypedMemoryRow);
    store.writeCount = 0;
    const capturedCandidates: SendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate[] = [];
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextParser: { enabled: true },
        response: {
          enabled: true,
          replyText: {
            enabled: true,
            deliveryCandidate: {
              enabled: true,
              record: (candidate) => {
                capturedCandidates.push(candidate);
              }
            }
          }
        }
      }
    });

    for (const content of [
      'review memory alert_timing',
      'explain memory alert_timing',
      'forget memory alert_timing'
    ]) {
      const r = await handleSendBlueInbound(
        { body: inboundBody({ content, message_handle: `sb-candidate-${content.split(' ')[0]}` }), secretHeaderValue: WEBHOOK_SECRET },
        h.deps
      );
      assert.equal(r.status, 200);
      assert.deepEqual(JSON.parse(r.body), { ok: true, intent: 'unclear', source: 'classifier' });
    }

    assert.equal(capturedCandidates.length, 3);
    assert.equal(capturedCandidates.every((candidate) => candidate.to === FOUNDER_PHONE), true);
    assert.equal(capturedCandidates.every((candidate) => candidate.text.length > 0), true);
    assert.deepEqual(
      capturedCandidates.map((candidate) => candidate.metadata.matched_intent),
      ['memory_review', 'memory_explanation', 'memory_forget']
    );
    const candidateMetadataJson = JSON.stringify(capturedCandidates.map((candidate) => candidate.metadata));
    assert.equal(candidateMetadataJson.includes(FOUNDER_PHONE), false);
    const responseAndAuditJson = JSON.stringify({
      audits: await h.auditStore.recent(FOUNDER_USER, 50),
      responseBodies: capturedCandidates.map((candidate) => candidate.metadata)
    });
    for (const forbidden of [
      FOUNDER_PHONE,
      'founder private mornings',
      'founder-private-candidate-ref',
      'other-user',
      'other-user private candidate evenings',
      'other-user-private-candidate-ref',
      'sendblue-visible-memory-exact-command',
      'review memory alert_timing',
      'explain memory alert_timing',
      'forget memory alert_timing'
    ]) {
      assert.equal(responseAndAuditJson.includes(forbidden), false, `delivery candidate seam leaked ${forbidden}`);
    }
  });

  it('reply delivery dispatch helper uses only an injected transport and returns sanitized metadata', async () => {
    const candidate = createSendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate({
      to: FOUNDER_PHONE,
      reply: {
        text: 'Safe delivery text from renderer.',
        metadata: {
          renderer: 'sendblue_visible_memory_command_reply_text_renderer',
          memory_kind: 'preference',
          handled: true,
          status: 'handled',
          matched_intent: 'memory_review',
          returned_count: 1
        }
      }
    });
    assert.notEqual(candidate, null);
    const transported: SendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate[] = [];

    const result = await dispatchSendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate({
      candidate: candidate!,
      transport: {
        dispatch: (dispatchCandidate) => {
          transported.push(dispatchCandidate);
        }
      }
    });

    assert.deepEqual(transported, [candidate]);
    assert.deepEqual(result, {
      dispatched: true,
      metadata: {
        dispatcher: 'sendblue_visible_memory_command_reply_delivery_dispatch',
        delivery_candidate_renderer: 'sendblue_visible_memory_command_reply_delivery_candidate',
        reply_text_renderer: 'sendblue_visible_memory_command_reply_text_renderer',
        memory_kind: 'preference',
        handled: true,
        status: 'handled',
        matched_intent: 'memory_review',
        returned_count: 1,
        to_slug: '3459',
        text_bytes: Buffer.byteLength('Safe delivery text from renderer.', 'utf8')
      }
    });
    const metadataJson = JSON.stringify(result.metadata);
    assert.equal(metadataJson.includes(FOUNDER_PHONE), false);
    assert.equal(metadataJson.includes('Safe delivery text from renderer.'), false);
  });

  it('reply delivery dispatch seam is inert by default and without an injected transport', async () => {
    const store = new CountingTypedMemoryStore();
    const transported: SendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate[] = [];
    const dispatchResults: SendBlueInboundVisibleMemoryCommandReplyDeliveryDispatchResult[] = [];
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextParser: { enabled: true },
        response: {
          enabled: true,
          replyText: {
            enabled: true,
            deliveryCandidate: {
              enabled: true,
              dispatch: {
                enabled: false,
                transport: {
                  dispatch: (candidate) => {
                    transported.push(candidate);
                  }
                },
                record: (result) => {
                  dispatchResults.push(result);
                }
              }
            }
          }
        }
      }
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: 'review memory alert_timing' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.deepEqual(JSON.parse(r.body), { ok: true, intent: 'unclear', source: 'classifier' });
    assert.deepEqual(transported, []);
    assert.deepEqual(dispatchResults, []);

    const hWithoutTransport = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store: new CountingTypedMemoryStore(),
        contextParser: { enabled: true },
        response: {
          enabled: true,
          replyText: {
            enabled: true,
            deliveryCandidate: {
              enabled: true,
              dispatch: {
                enabled: true,
                record: (result) => {
                  dispatchResults.push(result);
                }
              }
            }
          }
        }
      }
    });

    const rWithoutTransport = await handleSendBlueInbound(
      { body: inboundBody({ content: 'review memory alert_timing' }), secretHeaderValue: WEBHOOK_SECRET },
      hWithoutTransport.deps
    );

    assert.equal(rWithoutTransport.status, 200);
    assert.deepEqual(JSON.parse(rWithoutTransport.body), { ok: true, intent: 'unclear', source: 'classifier' });
    assert.deepEqual(transported, []);
    assert.deepEqual(dispatchResults, []);
  });

  it('when test-enabled, dispatches delivery candidates without exposing private values, dispatch text, source refs, inbound text, cross-user data, or full phone in metadata', async () => {
    const store = new CountingTypedMemoryStore();
    await store.write({
      user_id: FOUNDER_USER,
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'founder private dispatch mornings alice@example.com',
      source: 'user_stated',
      source_ref: 'reply:founder-private-dispatch-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T11:00:00.000Z'
    } as NewTypedMemoryRow);
    await store.write({
      user_id: 'other-user',
      kind: 'preference',
      scope_key: 'preference:alert_timing',
      attribute: 'alert_timing',
      value: 'other-user private dispatch evenings alice@example.com',
      source: 'user_stated',
      source_ref: 'reply:other-user-private-dispatch-ref',
      confidence: 'high',
      stale_marked_at: null,
      retracted: false,
      superseded_by: null,
      updated_at: '2026-07-09T12:00:00.000Z'
    } as NewTypedMemoryRow);
    store.writeCount = 0;
    const transported: SendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate[] = [];
    const dispatchResults: SendBlueInboundVisibleMemoryCommandReplyDeliveryDispatchResult[] = [];
    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store,
        contextParser: { enabled: true },
        response: {
          enabled: true,
          replyText: {
            enabled: true,
            deliveryCandidate: {
              enabled: true,
              dispatch: {
                enabled: true,
                transport: {
                  dispatch: (candidate) => {
                    transported.push(candidate);
                  }
                },
                record: (result) => {
                  dispatchResults.push(result);
                }
              }
            }
          }
        }
      }
    });

    for (const content of [
      'review memory alert_timing',
      'explain memory alert_timing',
      'forget memory alert_timing'
    ]) {
      const r = await handleSendBlueInbound(
        { body: inboundBody({ content, message_handle: `sb-dispatch-${content.split(' ')[0]}` }), secretHeaderValue: WEBHOOK_SECRET },
        h.deps
      );
      assert.equal(r.status, 200);
      assert.deepEqual(JSON.parse(r.body), { ok: true, intent: 'unclear', source: 'classifier' });
    }

    assert.equal(transported.length, 3);
    assert.equal(dispatchResults.length, 3);
    assert.deepEqual(
      dispatchResults.map((result) => result.metadata.matched_intent),
      ['memory_review', 'memory_explanation', 'memory_forget']
    );
    assert.equal(dispatchResults.every((result) => result.metadata.to_slug === '3459'), true);
    assert.equal(dispatchResults.every((result) => result.metadata.text_bytes > 0), true);
    const responseAndAuditJson = JSON.stringify({
      audits: await h.auditStore.recent(FOUNDER_USER, 50),
      dispatchMetadata: dispatchResults.map((result) => result.metadata)
    });
    for (const forbidden of [
      FOUNDER_PHONE,
      'founder private dispatch mornings',
      'founder-private-dispatch-ref',
      'other-user',
      'other-user private dispatch evenings',
      'other-user-private-dispatch-ref',
      'sendblue-visible-memory-exact-command',
      'review memory alert_timing',
      'explain memory alert_timing',
      'forget memory alert_timing',
      ...transported.map((candidate) => candidate.text)
    ]) {
      assert.equal(responseAndAuditJson.includes(forbidden), false, `delivery dispatch seam leaked ${forbidden}`);
    }
  });

  it('does not attempt delivery dispatch for empty, unhandled, or missing delivery candidates', async () => {
    const transported: SendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate[] = [];
    const dispatchResults: SendBlueInboundVisibleMemoryCommandReplyDeliveryDispatchResult[] = [];
    const baseReply: SendBlueInboundVisibleMemoryCommandReplyText = {
      text: 'safe text',
      metadata: {
        renderer: 'sendblue_visible_memory_command_reply_text_renderer',
        memory_kind: 'preference',
        handled: true,
        status: 'handled'
      }
    };

    for (const reply of [
      { ...baseReply, text: '   ' },
      { ...baseReply, metadata: { ...baseReply.metadata, handled: false, status: 'no_memory_command' as const } }
    ]) {
      const candidate = createSendBlueInboundVisibleMemoryCommandReplyDeliveryCandidate({
        to: FOUNDER_PHONE,
        reply
      });
      assert.equal(candidate, null);
    }

    const h = buildHarness({
      visibleMemoryCommand: {
        enabled: true,
        store: new CountingTypedMemoryStore(),
        contextParser: { enabled: true },
        response: {
          enabled: true,
          replyText: {
            enabled: true,
            deliveryCandidate: {
              enabled: true,
              create: () => null,
              dispatch: {
                enabled: true,
                transport: {
                  dispatch: (candidate) => {
                    transported.push(candidate);
                  }
                },
                record: (result) => {
                  dispatchResults.push(result);
                }
              }
            }
          }
        }
      }
    });

    const r = await handleSendBlueInbound(
      { body: inboundBody({ content: 'review memory alert_timing' }), secretHeaderValue: WEBHOOK_SECRET },
      h.deps
    );

    assert.equal(r.status, 200);
    assert.deepEqual(JSON.parse(r.body), { ok: true, intent: 'unclear', source: 'classifier' });
    assert.deepEqual(transported, []);
    assert.deepEqual(dispatchResults, []);
  });
});

/* ====================================================================== */
/* Privacy invariants (LOAD-BEARING)                                      */
/* ====================================================================== */

describe('handleSendBlueInbound — privacy invariants', () => {
  it('audit detail NEVER contains the founder reply text', async () => {
    const h = buildHarness();
    await seedSentAlert(h, 'alert-priv-1');
    const SECRET_REPLY = 'this is the secret reply text that must not appear in audit';
    h.deps.replyParser.parse = async () =>
      Object.freeze({
        ok: true as const,
        source: 'classifier' as const,
        classification: Object.freeze({
          intent: 'snooze' as const,
          confidence: 0.9,
          reason: 'snooze',
          snooze_hint: 'later' as const
        }),
        model_name: 'mock',
        prompt_version: 'reply-parser-v0.1.0',
        latency_ms: 1,
        input_tokens: 1,
        output_tokens: 1,
        estimated_cost_usd: 0,
        low_confidence_forced_unclear: false
      });
    const body = inboundBody({ content: SECRET_REPLY });
    await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const sysAudits = await h.auditStore.recent(null as unknown as string, 50);
    for (const e of [...audits, ...sysAudits]) {
      const detailStr = JSON.stringify(e.detail);
      assert.ok(
        !detailStr.includes(SECRET_REPLY),
        `audit ${e.action} leaked reply text in detail: ${detailStr}`
      );
    }
  });

  it('audit detail NEVER contains the full founder phone number', async () => {
    const h = buildHarness();
    const body = inboundBody({ content: 'tomorrow' });
    await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const sysAudits = await h.auditStore.recent(null as unknown as string, 50);
    for (const e of [...audits, ...sysAudits]) {
      const detailStr = JSON.stringify(e.detail);
      assert.ok(
        !detailStr.includes(FOUNDER_PHONE),
        `audit ${e.action} leaked full founder phone: ${detailStr}`
      );
    }
  });

  it('audit detail NEVER contains the SendBlue signing secret', async () => {
    const h = buildHarness();
    const body = inboundBody({ content: 'tomorrow' });
    await handleSendBlueInbound({ body, secretHeaderValue: WEBHOOK_SECRET }, h.deps);
    const audits = await h.auditStore.recent(FOUNDER_USER, 50);
    const sysAudits = await h.auditStore.recent(null as unknown as string, 50);
    for (const e of [...audits, ...sysAudits]) {
      const detailStr = JSON.stringify(e.detail);
      assert.ok(
        !detailStr.includes(WEBHOOK_SECRET),
        `audit ${e.action} leaked webhook secret: ${detailStr}`
      );
    }
  });
});
