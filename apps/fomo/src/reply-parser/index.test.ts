import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryCostStore } from '../core/cost-tracking.ts';
import { applyEgressForReplyParser } from '../core/egress-policy.ts';
import { MockModelBackend, normalizePrompt } from '../core/model-backends/mock.ts';
import { createModelRouter } from '../core/model-router.ts';

import {
  DEFAULT_CONFIDENCE_THRESHOLD,
  PROMPT_VERSION,
  computeSnoozeDurationSeconds,
  parseReply,
  type ReplyAlertContext
} from './index.ts';
import { buildReplyParserPrompt } from './prompt.ts';

const ALERT_CONTEXT: ReplyAlertContext = Object.freeze({
  alert_subject: 'Reminder: deposit due tonight',
  alert_sender_name: 'Sarah',
  alert_message_id: 'msg-abc'
});

function makeRouter() {
  const cost = new InMemoryCostStore();
  const router = createModelRouter({ costStore: cost });
  return { router, cost };
}

// Build the prompt for a given reply so the mock backend's response
// map can key on the exact prompt the classifier sends.
function promptFor(replyText: string): string {
  const view = applyEgressForReplyParser(replyText, {
    subject: ALERT_CONTEXT.alert_subject,
    sender_name: ALERT_CONTEXT.alert_sender_name,
    message_id: ALERT_CONTEXT.alert_message_id
  });
  return buildReplyParserPrompt(view);
}

/* ====================================================================== */
/* Pass 1: deterministic safety/control pre-pass                          */
/* ====================================================================== */

describe('parseReply — deterministic pre-pass (LLM NEVER consulted)', () => {
  it('STOP returns deterministic stop without touching the router', async () => {
    const { router, cost } = makeRouter();
    // No backend registered — if the orchestrator calls the router,
    // the call will fail with 'no_backend_for_capability'. The
    // deterministic path must not reach it.
    const result = await parseReply(
      { user_reply_text: 'STOP', alert_context: ALERT_CONTEXT, user_id: 'u-1' },
      { router }
    );
    assert.equal(result.ok, true);
    if (result.ok && result.source === 'deterministic') {
      assert.equal(result.intent, 'stop');
    } else {
      assert.fail(`expected deterministic stop, got ${JSON.stringify(result)}`);
    }
    // Verify zero cost was recorded (no model call).
    assert.equal((await cost.recent('u-1')).length, 0);
  });

  it('UNSUBSCRIBE / CANCEL / END / QUIT all map to deterministic stop', async () => {
    const { router } = makeRouter();
    for (const t of ['UNSUBSCRIBE', 'cancel', 'End', 'quit', 'stop!', 'CANCEL.']) {
      const r = await parseReply(
        { user_reply_text: t, alert_context: ALERT_CONTEXT, user_id: 'u-1' },
        { router }
      );
      assert.equal(r.ok, true, `expected ok for ${t}`);
      if (r.ok && r.source === 'deterministic') assert.equal(r.intent, 'stop');
    }
  });

  it('START / RESUME / UNSTOP all map to deterministic start', async () => {
    const { router } = makeRouter();
    for (const t of ['START', 'start', 'Resume!', 'unstop']) {
      const r = await parseReply(
        { user_reply_text: t, alert_context: ALERT_CONTEXT, user_id: 'u-1' },
        { router }
      );
      assert.equal(r.ok, true);
      if (r.ok && r.source === 'deterministic') assert.equal(r.intent, 'start');
    }
  });
});

/* ====================================================================== */
/* Pass 2: classifier (when deterministic does not match)                 */
/* ====================================================================== */

describe('parseReply — classifier path (soft intents)', () => {
  it('parses snooze-shaped reply → snooze with snooze_hint=tomorrow', async () => {
    // Phase v0.5.10 — reply must be >3 words to bypass the Q3.C ≤3-word
    // safe rule (and not match the explicit-feedback-phrase allowlist).
    const replyText = 'please snooze this until tomorrow morning';
    const { router, cost } = makeRouter();
    const backend = new MockModelBackend({
      model_name: 'mock-reply-parser',
      responses: {
        [normalizePrompt(promptFor(replyText))]: {
          text: JSON.stringify({
            intent: 'snooze',
            confidence: 0.95,
            reason: 'user asked for tomorrow',
            snooze_hint: 'tomorrow'
          }),
          input_tokens: 50,
          output_tokens: 20,
          latency_ms: 10
        }
      }
    });
    router.registerBackend('classification', backend);

    const result = await parseReply(
      { user_reply_text: replyText, alert_context: ALERT_CONTEXT, user_id: 'u-1' },
      { router }
    );
    assert.equal(result.ok, true);
    if (result.ok && result.source === 'classifier') {
      assert.equal(result.classification.intent, 'snooze');
      assert.equal(result.classification.snooze_hint, 'tomorrow');
      assert.equal(result.classification.confidence, 0.95);
      assert.equal(result.low_confidence_forced_unclear, false);
      assert.equal(result.prompt_version, PROMPT_VERSION);
      assert.equal(result.model_name, 'mock-reply-parser');
    } else {
      assert.fail('expected classifier path');
    }
    // Verify cost WAS recorded.
    assert.ok((await cost.recent('u-1')).length > 0);
  });

  it('parses "why" → why with snooze_hint=null', async () => {
    const replyText = 'why this one?';
    const { router } = makeRouter();
    router.registerBackend(
      'classification',
      new MockModelBackend({
        model_name: 'mock',
        responses: {
          [normalizePrompt(promptFor(replyText))]: {
            text: JSON.stringify({
              intent: 'why',
              confidence: 0.92,
              reason: 'asked',
              snooze_hint: null
            }),
            input_tokens: 30,
            output_tokens: 12,
            latency_ms: 5
          }
        }
      })
    );
    const r = await parseReply(
      { user_reply_text: replyText, alert_context: ALERT_CONTEXT, user_id: 'u-1' },
      { router }
    );
    assert.equal(r.ok, true);
    if (r.ok && r.source === 'classifier') {
      assert.equal(r.classification.intent, 'why');
      assert.equal(r.classification.snooze_hint, null);
    }
  });

  it('parses "ignore sender" → ignore_sender', async () => {
    const replyText = 'never alert me about Sarah again';
    const { router } = makeRouter();
    router.registerBackend(
      'classification',
      new MockModelBackend({
        model_name: 'mock',
        responses: {
          [normalizePrompt(promptFor(replyText))]: {
            text: JSON.stringify({
              intent: 'ignore_sender',
              confidence: 0.88,
              reason: 'sender suppression',
              snooze_hint: null
            }),
            input_tokens: 35,
            output_tokens: 15,
            latency_ms: 5
          }
        }
      })
    );
    const r = await parseReply(
      { user_reply_text: replyText, alert_context: ALERT_CONTEXT, user_id: 'u-1' },
      { router }
    );
    assert.equal(r.ok, true);
    if (r.ok && r.source === 'classifier') {
      assert.equal(r.classification.intent, 'ignore_sender');
    }
  });
});

/* ====================================================================== */
/* Pass 3: confidence-threshold fail-safe                                 */
/* ====================================================================== */

describe('parseReply — confidence-threshold fail-safe (load-bearing safety)', () => {
  it('forces intent to "unclear" when classifier confidence < 0.7 (default)', async () => {
    // Phase v0.5.10 — reply must be >3 words to bypass the ≤3-word safe
    // rule so we exercise ONLY the confidence-threshold mechanism here.
    const replyText = 'maybe later when convenient probably';
    const { router } = makeRouter();
    router.registerBackend(
      'classification',
      new MockModelBackend({
        model_name: 'mock',
        responses: {
          [normalizePrompt(promptFor(replyText))]: {
            text: JSON.stringify({
              intent: 'snooze',     // model picked something
              confidence: 0.4,      // but with low confidence
              reason: 'guessing',
              snooze_hint: 'later'
            }),
            input_tokens: 20,
            output_tokens: 12,
            latency_ms: 5
          }
        }
      })
    );
    const r = await parseReply(
      { user_reply_text: replyText, alert_context: ALERT_CONTEXT, user_id: 'u-1' },
      { router }
    );
    assert.equal(r.ok, true);
    if (r.ok && r.source === 'classifier') {
      // FORCED to unclear because confidence was 0.4 < 0.7.
      assert.equal(r.classification.intent, 'unclear');
      // snooze_hint is null even though the model returned 'later'.
      assert.equal(r.classification.snooze_hint, null);
      // The flag tells ops WHY we forced unclear.
      assert.equal(r.low_confidence_forced_unclear, true);
      assert.match(r.classification.reason, /forced_unclear/);
    } else {
      assert.fail('expected classifier path');
    }
  });

  it('does NOT force unclear when classifier already chose unclear', async () => {
    const replyText = 'asdf qwer zxcv';
    const { router } = makeRouter();
    router.registerBackend(
      'classification',
      new MockModelBackend({
        model_name: 'mock',
        responses: {
          [normalizePrompt(promptFor(replyText))]: {
            text: JSON.stringify({
              intent: 'unclear',
              confidence: 0.3,
              reason: 'gibberish',
              snooze_hint: null
            }),
            input_tokens: 15,
            output_tokens: 12,
            latency_ms: 5
          }
        }
      })
    );
    const r = await parseReply(
      { user_reply_text: replyText, alert_context: ALERT_CONTEXT, user_id: 'u-1' },
      { router }
    );
    assert.equal(r.ok, true);
    if (r.ok && r.source === 'classifier') {
      assert.equal(r.classification.intent, 'unclear');
      assert.equal(r.low_confidence_forced_unclear, false);
      assert.match(r.classification.reason, /gibberish/);
    }
  });

  it('respects a custom confidenceThreshold (smoke tunability)', async () => {
    // Phase v0.5.10 — >3 words so the ≤3-word safe rule doesn't fire.
    const replyText = 'maybe push this to later please';
    const { router } = makeRouter();
    router.registerBackend(
      'classification',
      new MockModelBackend({
        model_name: 'mock',
        responses: {
          [normalizePrompt(promptFor(replyText))]: {
            text: JSON.stringify({
              intent: 'snooze',
              confidence: 0.6,
              reason: 'tentative snooze',
              snooze_hint: 'later'
            }),
            input_tokens: 20,
            output_tokens: 12,
            latency_ms: 5
          }
        }
      })
    );
    // With default threshold (0.7), 0.6 should force unclear.
    const r1 = await parseReply(
      { user_reply_text: replyText, alert_context: ALERT_CONTEXT, user_id: 'u-1' },
      { router }
    );
    assert.equal(r1.ok, true);
    if (r1.ok && r1.source === 'classifier') {
      assert.equal(r1.classification.intent, 'unclear');
    }
    // With a lower threshold (0.5), 0.6 should pass through.
    const r2 = await parseReply(
      { user_reply_text: replyText, alert_context: ALERT_CONTEXT, user_id: 'u-1' },
      { router, confidenceThreshold: 0.5 }
    );
    assert.equal(r2.ok, true);
    if (r2.ok && r2.source === 'classifier') {
      assert.equal(r2.classification.intent, 'snooze');
      assert.equal(r2.classification.snooze_hint, 'later');
    }
  });
});

/* ====================================================================== */
/* Classifier failure → orchestrator returns failure                      */
/* ====================================================================== */

describe('parseReply — classifier failure paths', () => {
  it('returns failure when no backend is registered for classification', async () => {
    const { router } = makeRouter();
    const r = await parseReply(
      { user_reply_text: 'tomorrow', alert_context: ALERT_CONTEXT, user_id: 'u-1' },
      { router }
    );
    assert.equal(r.ok, false);
    if (!r.ok) {
      assert.equal(r.source, 'classifier_error');
      assert.equal(r.code, 'no_backend_for_capability');
    }
  });

  it('returns failure when classifier returns invalid JSON (schema_invalid)', async () => {
    const replyText = 'something';
    const { router } = makeRouter();
    router.registerBackend(
      'classification',
      new MockModelBackend({
        model_name: 'mock',
        responses: {
          [normalizePrompt(promptFor(replyText))]: {
            text: 'not json',
            input_tokens: 10,
            output_tokens: 5,
            latency_ms: 2
          }
        }
      })
    );
    const r = await parseReply(
      { user_reply_text: replyText, alert_context: ALERT_CONTEXT, user_id: 'u-1' },
      { router }
    );
    assert.equal(r.ok, false);
    if (!r.ok) {
      assert.equal(r.source, 'classifier_error');
      assert.equal(r.code, 'schema_invalid');
    }
  });
});

/* ====================================================================== */
/* Egress invariant — load-bearing privacy check                          */
/* ====================================================================== */

describe('parseReply — egress invariant (no PII / no email body in prompt)', () => {
  it('classifier prompt never contains the egress canary fields', async () => {
    // Phase v0.5.10 — reply must be >3 words to bypass Q3.C ≤3-word
    // safe rule and exercise the classifier path.
    const replyText = 'please snooze until tomorrow';
    let promptSeenByBackend = '';
    const { router } = makeRouter();
    // Custom backend that captures the prompt the router sent.
    router.registerBackend('classification', {
      name() {
        return 'capture-backend';
      },
      async call(req: { prompt: string }) {
        promptSeenByBackend = req.prompt;
        return {
          text: JSON.stringify({
            intent: 'snooze',
            confidence: 0.9,
            reason: 'ok',
            snooze_hint: 'tomorrow'
          }),
          input_tokens: 1,
          output_tokens: 1,
          latency_ms: 1,
          model_name: 'capture-backend'
        };
      }
    });
    const r = await parseReply(
      { user_reply_text: replyText, alert_context: ALERT_CONTEXT, user_id: 'u-1' },
      { router }
    );
    assert.equal(r.ok, true);
    // The prompt MUST contain the reply text (it's the whole input).
    assert.ok(promptSeenByBackend.includes(replyText));
    // It MUST NOT contain any canary that would indicate egress leaked.
    // (ReplyParserEgressView intentionally has no field for raw body /
    // headers — these never reach the parser. Defensive assertion.)
    for (const forbidden of ['body_plain', 'body_html', 'Received:', 'attachment', 'X-Probe']) {
      assert.ok(
        !promptSeenByBackend.includes(forbidden),
        `prompt leaked forbidden token: ${forbidden}`
      );
    }
  });
});

/* ====================================================================== */
/* computeSnoozeDurationSeconds — mapping table                           */
/* ====================================================================== */

describe('computeSnoozeDurationSeconds', () => {
  it('maps later → 1h, tomorrow → 24h, remind_me_later → 4h, unspecified/null → 1h default', () => {
    assert.equal(computeSnoozeDurationSeconds('later'), 60 * 60);
    assert.equal(computeSnoozeDurationSeconds('tomorrow'), 24 * 60 * 60);
    assert.equal(computeSnoozeDurationSeconds('remind_me_later'), 4 * 60 * 60);
    assert.equal(computeSnoozeDurationSeconds('unspecified'), 60 * 60);
    assert.equal(computeSnoozeDurationSeconds(null), 60 * 60);
  });
});

describe('DEFAULT_CONFIDENCE_THRESHOLD', () => {
  it('is 0.7 (founder directive: fail safe on low confidence)', () => {
    assert.equal(DEFAULT_CONFIDENCE_THRESHOLD, 0.7);
  });
});
