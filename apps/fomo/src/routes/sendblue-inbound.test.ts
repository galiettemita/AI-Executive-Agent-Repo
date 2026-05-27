import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAlertStateTransitionStore } from '../core/alert-state-transitions.ts';
import { InMemoryAuditStore } from '../core/audit.ts';
import { SAFE_DEFAULT_KILL_SWITCHES, type KillSwitches } from '../core/kill-switches.ts';
import { InMemoryAlertStore } from '../memory/alerts.ts';
import { InMemoryFeedbackStore } from '../memory/feedback-events.ts';
import { InMemoryInboundReplyStore } from '../memory/inbound-replies.ts';
import { InMemoryMemorySignalStore } from '../memory/memory-signals.ts';
import { InMemoryRankResultStore } from '../memory/rank-results.ts';
import {
  type ReplyParseResult,
  type ReplyParserRequest
} from '../reply-parser/index.ts';
import {
  handleSendBlueInbound,
  type SendBlueInboundRouteDeps
} from './sendblue-inbound.ts';

const WEBHOOK_SECRET = 'shh-test-sb-webhook-secret-from-dashboard';
const WEBHOOK_SECRET_HEADER = 'sb-signing-secret';
const FOUNDER_PHONE = '+16467023459';
const FOUNDER_USER = 'founder';


interface HarnessOverrides {
  readonly killSwitches?: KillSwitches;
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
    replyParser: { parse: parser }
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
    // Feedback event written with snooze_until.
    const fb = await h.feedbackStore.recent(FOUNDER_USER, 20);
    const snoozeFb = fb.find((e) => e.kind === 'user_snoozed');
    assert.ok(snoozeFb);
    assert.equal(snoozeFb?.alert_id, 'alert-snooze-1');
    const detail = snoozeFb?.detail as { snooze_until: string; snooze_hint: string };
    assert.ok(detail.snooze_until);
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
    assert.equal(await h.feedbackStore.countByKind(FOUNDER_USER, 'ignored_sender'), 1);
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
