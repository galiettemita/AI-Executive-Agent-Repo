// Phase v0.5.14 — Feedback acknowledgment renderer unit tests.
//
// What unit tests prove for this surface (per [[risk-tiered-verification]]
// surface-scope rule): each intent maps to a single deterministic body;
// the bodies match the founder-locked text exactly; privacy canary
// catches any future drift that would leak sender / subject / body
// fragments; the renderer is pure and frozen.
//
// What unit tests CANNOT prove (deferred to founder taste check pre-
// commit + the existing SendBlue integration path):
//   * Whether the body "feels like a helpful person" — that's the taste
//     check the founder approved 2026-06-07 before this commit landed.
//   * Provider delivery — already proven by the alert + STOP outbound
//     paths; this surface adds another message type, not a new wire.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  FEEDBACK_ACK_TEMPLATE_VERSION,
  _ACK_BODIES_FOR_TESTS,
  isAckableFeedbackIntent,
  renderFeedbackAck,
  type AckableFeedbackIntent
} from './feedback-ack-template.ts';

describe('renderFeedbackAck — deterministic per-intent text', () => {
  it('returns the founder-locked body for ignore_sender', () => {
    const r = renderFeedbackAck('ignore_sender');
    assert.equal(r.body, "Got it — I'll quiet that sender.");
    assert.equal(r.template_version, FEEDBACK_ACK_TEMPLATE_VERSION);
  });

  it('returns the founder-locked body for this_mattered', () => {
    const r = renderFeedbackAck('this_mattered');
    assert.equal(r.body, "Thanks — I'll remember this was worth catching.");
    assert.equal(r.template_version, FEEDBACK_ACK_TEMPLATE_VERSION);
  });

  it('returns the founder-locked body for more_like_this', () => {
    const r = renderFeedbackAck('more_like_this');
    assert.equal(r.body, "Got it — I'll catch more like that.");
    assert.equal(r.template_version, FEEDBACK_ACK_TEMPLATE_VERSION);
  });

  it('returns the founder-locked body for false_positive', () => {
    const r = renderFeedbackAck('false_positive');
    assert.equal(r.body, "Got it — I'll be more careful with emails like that.");
    assert.equal(r.template_version, FEEDBACK_ACK_TEMPLATE_VERSION);
  });

  it('is deterministic — same input always produces same output', () => {
    for (const intent of ['ignore_sender', 'this_mattered', 'more_like_this', 'false_positive'] as const) {
      const a = renderFeedbackAck(intent);
      const b = renderFeedbackAck(intent);
      assert.equal(a.body, b.body);
      assert.equal(a.template_version, b.template_version);
    }
  });

  it('template version is the locked v0.5.14 string', () => {
    assert.equal(FEEDBACK_ACK_TEMPLATE_VERSION, 'feedback-ack-v0.1.0');
  });

  it('returned object is frozen — caller cannot mutate body', () => {
    const r = renderFeedbackAck('ignore_sender');
    assert.throws(() => {
      (r as unknown as { body: string }).body = 'tampered';
    });
  });

  it('underlying ACK_BODIES table is frozen', () => {
    assert.throws(() => {
      (_ACK_BODIES_FOR_TESTS as unknown as { ignore_sender: string }).ignore_sender = 'tampered';
    });
  });
});

describe('renderFeedbackAck — bounded length contract', () => {
  // The founder lock is 6–12 words, single sentence. Test the bounded
  // contract so a future drift past the bound surfaces as a test
  // failure rather than a silent regression of [[brevio-voice-rules]].
  const ALL_INTENTS: readonly AckableFeedbackIntent[] = [
    'ignore_sender',
    'this_mattered',
    'more_like_this',
    'false_positive'
  ];

  for (const intent of ALL_INTENTS) {
    it(`${intent}: body is 6–12 words inclusive`, () => {
      const body = renderFeedbackAck(intent).body;
      const wordCount = body.split(/\s+/).filter((w) => w.length > 0).length;
      assert.ok(wordCount >= 6, `${intent} body has ${wordCount} words; minimum 6: "${body}"`);
      assert.ok(wordCount <= 12, `${intent} body has ${wordCount} words; maximum 12: "${body}"`);
    });

    it(`${intent}: body is single sentence (one period, no semicolons)`, () => {
      const body = renderFeedbackAck(intent).body;
      // Single trailing period (em-dashes are fine, no internal periods)
      const periodCount = (body.match(/\./g) ?? []).length;
      assert.equal(periodCount, 1, `${intent} body must have exactly one period: "${body}"`);
      assert.ok(!body.includes(';'), `${intent} body must not contain semicolons: "${body}"`);
    });

    it(`${intent}: body ends with a period (no trailing whitespace or punctuation)`, () => {
      const body = renderFeedbackAck(intent).body;
      assert.ok(body.endsWith('.'), `${intent} body must end with a period: "${body}"`);
      assert.ok(!body.endsWith(' .'), `${intent} body must not have whitespace before the period: "${body}"`);
    });
  }
});

describe('renderFeedbackAck — privacy canary (no PII / metadata leak)', () => {
  // Surface-scope rule: the renderer takes ONLY the intent enum. Even
  // a future regression where someone interpolates a sender email into
  // a body should fail this canary because the bodies are static
  // strings. The canary is paranoia: it future-proofs the contract.
  const FORBIDDEN_SUBSTRINGS = [
    // Email TLDs and full-domain hints
    '@gmail.com',
    '@icloud.com',
    '@hotmail.com',
    '@yahoo.com',
    '@example.com',
    'noreply@',
    'unsubscribe',
    // Header / field shape
    'Subject:',
    'From:',
    'To:',
    'Reply-To:',
    // Metadata mentions the renderer should never produce
    'score',
    'rank',
    'PIL',
    'sender_email',
    'message_id',
    // Reply-text leak markers ([[dont-oversell-renderer-tone]] +
    // privacy guardrails — never echo the user's wording back)
    'you said',
    'you wrote',
    'you replied'
  ] as const;

  const ALL_INTENTS: readonly AckableFeedbackIntent[] = [
    'ignore_sender',
    'this_mattered',
    'more_like_this',
    'false_positive'
  ];

  for (const intent of ALL_INTENTS) {
    it(`${intent}: body contains zero forbidden substrings`, () => {
      const body = renderFeedbackAck(intent).body;
      const lower = body.toLowerCase();
      const hits: string[] = [];
      for (const needle of FORBIDDEN_SUBSTRINGS) {
        if (lower.includes(needle.toLowerCase())) hits.push(needle);
      }
      assert.deepEqual(
        hits,
        [],
        `${intent} body must not contain any forbidden substring. Hits: ${JSON.stringify(hits)}. Body: "${body}"`
      );
    });
  }
});

describe('isAckableFeedbackIntent — set membership', () => {
  it('returns true for each of the 4 ackable intents', () => {
    assert.equal(isAckableFeedbackIntent('ignore_sender'), true);
    assert.equal(isAckableFeedbackIntent('this_mattered'), true);
    assert.equal(isAckableFeedbackIntent('more_like_this'), true);
    assert.equal(isAckableFeedbackIntent('false_positive'), true);
  });

  it('returns false for stop / start / unclear (compliance + short-circuit paths)', () => {
    assert.equal(isAckableFeedbackIntent('stop'), false);
    assert.equal(isAckableFeedbackIntent('start'), false);
    assert.equal(isAckableFeedbackIntent('unclear'), false);
  });

  it('returns false for v0.5.10 declared intents NOT in the v0.5.14 ackable set', () => {
    // snooze / ignore / why are declared in the parser but not acked
    // until their own tier classification considers it.
    assert.equal(isAckableFeedbackIntent('snooze'), false);
    assert.equal(isAckableFeedbackIntent('ignore'), false);
    assert.equal(isAckableFeedbackIntent('why'), false);
  });

  it('returns false for arbitrary unknown strings (defense-in-depth)', () => {
    assert.equal(isAckableFeedbackIntent(''), false);
    assert.equal(isAckableFeedbackIntent('unknown_future_intent'), false);
    assert.equal(isAckableFeedbackIntent('IGNORE_SENDER'), false); // case-sensitive
  });
});
