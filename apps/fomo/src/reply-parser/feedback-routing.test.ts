// Phase v0.5.10 — feedback-routing module tests.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAuditStore } from '../core/audit.ts';
import { InMemoryFeedbackStore } from '../memory/feedback-events.ts';
import { InMemoryMemorySignalStore } from '../memory/memory-signals.ts';

import {
  _INTENT_MAPPING_FOR_TESTS,
  routeReplyFeedback,
  type RouteReplyFeedbackInput
} from './feedback-routing.ts';

const TEST_HASH_KEY = Buffer.alloc(32, 0xab);

function makeInput(over: Partial<RouteReplyFeedbackInput> = {}): RouteReplyFeedbackInput {
  return {
    user_id: over.user_id ?? 'founder',
    intent: over.intent ?? 'ignore',
    intent_source: over.intent_source ?? 'reply_parser_classifier',
    parser_confidence: 'parser_confidence' in over ? over.parser_confidence! : 0.9,
    inbound_reply_id: 'inbound_reply_id' in over ? over.inbound_reply_id! : 42,
    alert_id: 'alert_id' in over ? over.alert_id ?? null : 'alert-1',
    sender_email: 'sender_email' in over ? over.sender_email ?? null : null,
    snooze_hint: over.snooze_hint
  };
}

function makeDeps() {
  return {
    feedbackStore: new InMemoryFeedbackStore(),
    auditStore: new InMemoryAuditStore(),
    memoryStore: new InMemoryMemorySignalStore(),
    senderHashKey: TEST_HASH_KEY
  };
}

/* ====================================================================== */
/* Intent mapping table — locked per Q2.A-modified                        */
/* ====================================================================== */

describe('v0.5.10 INTENT_MAPPING — locked Q2.A-modified shapes', () => {
  it('snooze → verb=snoozed, dimension=alert, role=user, NO apply', () => {
    const m = _INTENT_MAPPING_FOR_TESTS.snooze;
    assert.equal(m.verb, 'snoozed');
    assert.equal(m.detail_overlay.dimension, 'alert');
    assert.equal(m.detail_overlay.role, 'user');
    assert.equal(m.fires_apply_feedback, false);
  });

  it('ignore → verb=ignored, dimension=alert, role=user, NO apply', () => {
    const m = _INTENT_MAPPING_FOR_TESTS.ignore;
    assert.equal(m.verb, 'ignored');
    assert.equal(m.detail_overlay.dimension, 'alert');
    assert.equal(m.fires_apply_feedback, false);
  });

  it('ignore_sender → verb=ignored, dimension=sender, role=user, FIRES apply (v0.5.9 consumer arm)', () => {
    const m = _INTENT_MAPPING_FOR_TESTS.ignore_sender;
    assert.equal(m.verb, 'ignored');
    assert.equal(m.detail_overlay.dimension, 'sender');
    assert.equal(m.fires_apply_feedback, true); // THE ONLY v0.5.10 fires_apply_feedback=true intent
  });

  it('why → verb=asked_why, role=user, NO apply', () => {
    const m = _INTENT_MAPPING_FOR_TESTS.why;
    assert.equal(m.verb, 'asked_why');
    assert.equal(m.fires_apply_feedback, false);
  });

  it('false_positive → verb=corrected, dimension=ranker_label, previous_label=important, corrected_label=not_important (founder v0.5.10 vocab)', () => {
    const m = _INTENT_MAPPING_FOR_TESTS.false_positive;
    assert.equal(m.verb, 'corrected');
    assert.equal(m.detail_overlay.dimension, 'ranker_label');
    assert.equal(m.detail_overlay.previous_label, 'important');
    assert.equal(m.detail_overlay.corrected_label, 'not_important');
    assert.equal(m.fires_apply_feedback, false);
  });

  it('this_mattered → verb=approved, dimension=importance, value=confirmed_important (POSITIVE confirmation; NOT a correction)', () => {
    const m = _INTENT_MAPPING_FOR_TESTS.this_mattered;
    assert.equal(m.verb, 'approved');
    assert.equal(m.detail_overlay.dimension, 'importance');
    assert.equal(m.detail_overlay.value, 'confirmed_important');
    assert.equal(m.detail_overlay.role, 'user');
    assert.equal(m.fires_apply_feedback, false);
  });

  it('more_like_this → verb=approved, dimension=pattern, value=more_like_this', () => {
    const m = _INTENT_MAPPING_FOR_TESTS.more_like_this;
    assert.equal(m.verb, 'approved');
    assert.equal(m.detail_overlay.dimension, 'pattern');
    assert.equal(m.detail_overlay.value, 'more_like_this');
    assert.equal(m.fires_apply_feedback, false);
  });

  it('Q4.A invariant: ONLY ignore_sender has fires_apply_feedback=true in v0.5.10', () => {
    const firing = Object.entries(_INTENT_MAPPING_FOR_TESTS)
      .filter(([, m]) => m.fires_apply_feedback)
      .map(([k]) => k);
    assert.deepEqual(firing, ['ignore_sender']);
  });
});

/* ====================================================================== */
/* routeReplyFeedback — happy-path writes                                 */
/* ====================================================================== */

describe('routeReplyFeedback — feedback_event + audit emission', () => {
  it('snooze → writes feedback_event(verb=snoozed, dimension=alert, snooze_hint=tomorrow); feedback.written audit detail carries all 10 fields; NO applyFeedback', async () => {
    const deps = makeDeps();
    const outcome = await routeReplyFeedback(
      makeInput({ intent: 'snooze', snooze_hint: 'tomorrow' }),
      deps
    );
    assert.equal(outcome.kind, 'wrote');
    if (outcome.kind !== 'wrote') return;
    assert.equal(outcome.applied, null); // Q4.A — only ignore_sender fires apply

    const events = await deps.feedbackStore.recent('founder');
    assert.equal(events.length, 1);
    assert.equal(events[0]!.kind, 'snoozed');
    assert.equal(events[0]!.source_surface, 'email_alert');
    const d = events[0]!.detail as Record<string, unknown>;
    assert.equal(d.dimension, 'alert');
    assert.equal(d.role, 'user');
    assert.equal(d.snooze_hint, 'tomorrow');

    // feedback.written audit detail.
    const audits = await deps.auditStore.recent('founder');
    const written = audits.find((a) => a.action === 'feedback.written');
    assert.ok(written);
    const ad = written.detail as Record<string, unknown>;
    assert.equal(ad.verb, 'snoozed');
    assert.equal(ad.dimension, 'alert');
    assert.equal(ad.role, 'user');
    assert.equal(ad.intent_source, 'reply_parser_classifier');
    assert.equal(ad.inbound_reply_id, 42);
    assert.equal(ad.parser_intent, 'snooze');
    assert.equal(ad.parser_confidence, 0.9);
    assert.equal(ad.source_surface, 'email_alert');
    assert.ok(typeof ad.feedback_event_id === 'number');
  });

  it('this_mattered → writes feedback_event(verb=approved, dimension=importance, value=confirmed_important); NO applyFeedback', async () => {
    const deps = makeDeps();
    const outcome = await routeReplyFeedback(
      makeInput({ intent: 'this_mattered' }),
      deps
    );
    assert.equal(outcome.kind, 'wrote');
    if (outcome.kind !== 'wrote') return;
    assert.equal(outcome.applied, null);

    const events = await deps.feedbackStore.recent('founder');
    const d = events[0]!.detail as Record<string, unknown>;
    assert.equal(events[0]!.kind, 'approved');
    assert.equal(d.dimension, 'importance');
    assert.equal(d.value, 'confirmed_important');
    assert.equal(d.role, 'user');

    // NO sender_feedback_ignored memory_signal write.
    assert.equal((await deps.memoryStore.list('founder')).length, 0);
  });

  it('more_like_this → writes feedback_event(verb=approved, dimension=pattern, value=more_like_this); NO applyFeedback', async () => {
    const deps = makeDeps();
    const outcome = await routeReplyFeedback(
      makeInput({ intent: 'more_like_this' }),
      deps
    );
    assert.equal(outcome.kind, 'wrote');
    const events = await deps.feedbackStore.recent('founder');
    const d = events[0]!.detail as Record<string, unknown>;
    assert.equal(events[0]!.kind, 'approved');
    assert.equal(d.dimension, 'pattern');
    assert.equal(d.value, 'more_like_this');
    assert.equal((await deps.memoryStore.list('founder')).length, 0);
  });

  it('false_positive → writes feedback_event(verb=corrected, dimension=ranker_label, previous_label=important, corrected_label=not_important)', async () => {
    const deps = makeDeps();
    await routeReplyFeedback(makeInput({ intent: 'false_positive' }), deps);
    const events = await deps.feedbackStore.recent('founder');
    const d = events[0]!.detail as Record<string, unknown>;
    assert.equal(events[0]!.kind, 'corrected');
    assert.equal(d.dimension, 'ranker_label');
    assert.equal(d.previous_label, 'important');
    assert.equal(d.corrected_label, 'not_important');
  });

  it('ignore_sender WITH sender_email → invokes applyFeedback → sender_feedback_ignored upsert + brevio.feedback.applied audit', async () => {
    const deps = makeDeps();
    const outcome = await routeReplyFeedback(
      makeInput({ intent: 'ignore_sender', sender_email: 'noisy@example-smoke.test' }),
      deps
    );
    assert.equal(outcome.kind, 'wrote');
    if (outcome.kind !== 'wrote') return;
    assert.ok(outcome.applied);
    assert.equal(outcome.applied!.kind, 'applied');

    // memory_signal upserted.
    const signals = await deps.memoryStore.list('founder');
    assert.equal(signals.length, 1);
    assert.equal(signals[0]!.kind, 'sender_feedback_ignored');
    assert.equal(signals[0]!.scope_key!.length, 32); // HMAC 32 hex chars

    // brevio.feedback.applied audit fires.
    const audits = await deps.auditStore.recent('founder');
    const applied = audits.find((a) => a.action === 'brevio.feedback.applied');
    assert.ok(applied);
  });

  it('ignore_sender WITHOUT sender_email → feedback_event written, applyFeedback returns no_match (graceful)', async () => {
    const deps = makeDeps();
    const outcome = await routeReplyFeedback(
      makeInput({ intent: 'ignore_sender', sender_email: null }),
      deps
    );
    assert.equal(outcome.kind, 'wrote');
    if (outcome.kind !== 'wrote') return;
    assert.ok(outcome.applied);
    assert.equal(outcome.applied!.kind, 'no_match');
    // Feedback_event still written.
    const events = await deps.feedbackStore.recent('founder');
    assert.equal(events.length, 1);
    assert.equal(events[0]!.kind, 'ignored');
    // No memory_signal upsert because no sender.
    assert.equal((await deps.memoryStore.list('founder')).length, 0);
  });

  it('deterministic intent_source → parser_confidence=1.0 surfaces in audit', async () => {
    const deps = makeDeps();
    await routeReplyFeedback(
      makeInput({ intent: 'this_mattered', intent_source: 'reply_parser_deterministic', parser_confidence: 1.0 }),
      deps
    );
    const audits = await deps.auditStore.recent('founder');
    const written = audits.find((a) => a.action === 'feedback.written');
    assert.ok(written);
    const ad = written.detail as Record<string, unknown>;
    assert.equal(ad.intent_source, 'reply_parser_deterministic');
    assert.equal(ad.parser_confidence, 1.0);
  });
});

/* ====================================================================== */
/* Cross-tenant + privacy canary                                          */
/* ====================================================================== */

describe('routeReplyFeedback — cross-tenant + privacy', () => {
  it('cross-tenant: write for user A does NOT touch user B feedback_events or memory_signals', async () => {
    const deps = makeDeps();
    await routeReplyFeedback(makeInput({ user_id: 'userA', intent: 'ignore_sender', sender_email: 'noisy@example-smoke.test' }), deps);
    assert.equal((await deps.feedbackStore.recent('userA')).length, 1);
    assert.equal((await deps.feedbackStore.recent('userB')).length, 0);
    assert.equal((await deps.memoryStore.list('userA')).length, 1);
    assert.equal((await deps.memoryStore.list('userB')).length, 0);
  });

  it('privacy canary: NO raw reply text / subject / body / sender_email substring in audit detail (the reply text is never passed to the routing module)', async () => {
    const deps = makeDeps();
    await routeReplyFeedback(
      makeInput({ intent: 'ignore_sender', sender_email: 'noisy-newsletter@example-smoke.test' }),
      deps
    );
    const audits = await deps.auditStore.recent('founder');
    for (const a of audits) {
      const json = JSON.stringify(a.detail ?? {});
      // The structured metadata is allowed; raw sender_email is NOT
      // present in the new audit detail (it's hashed in
      // memory_signal_scope_key_hash, NOT echoed back to feedback.written).
      assert.equal(json.includes('noisy-newsletter@example-smoke.test'), false);
      assert.equal(json.includes('Subject:'), false);
      assert.equal(json.includes('body_plain'), false);
    }
  });
});
