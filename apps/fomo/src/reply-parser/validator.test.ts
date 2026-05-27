import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  REPLY_INTENTS,
  SNOOZE_HINTS_NON_NULL,
  isReplyIntent,
  validateReplyClassifierOutput
} from './validator.ts';

describe('isReplyIntent', () => {
  it('accepts every declared intent', () => {
    for (const i of REPLY_INTENTS) {
      assert.equal(isReplyIntent(i), true);
    }
  });

  it('rejects unknown strings and non-strings', () => {
    assert.equal(isReplyIntent('stop'), false); // compliance intent — handled deterministically, NOT a classifier output
    assert.equal(isReplyIntent('start'), false);
    assert.equal(isReplyIntent(''), false);
    assert.equal(isReplyIntent(null), false);
    assert.equal(isReplyIntent(undefined), false);
    assert.equal(isReplyIntent(42), false);
  });
});

describe('validateReplyClassifierOutput — happy path', () => {
  it('parses a minimal valid snooze classification', () => {
    const text = JSON.stringify({
      intent: 'snooze',
      confidence: 0.92,
      reason: 'user said tomorrow',
      snooze_hint: 'tomorrow'
    });
    const r = validateReplyClassifierOutput(text);
    assert.equal(r.ok, true);
    if (r.ok) {
      assert.equal(r.value.intent, 'snooze');
      assert.equal(r.value.confidence, 0.92);
      assert.equal(r.value.snooze_hint, 'tomorrow');
    }
  });

  it('accepts intent=ignore with snooze_hint=null', () => {
    const text = JSON.stringify({
      intent: 'ignore',
      confidence: 0.8,
      reason: 'user said skip',
      snooze_hint: null
    });
    const r = validateReplyClassifierOutput(text);
    assert.equal(r.ok, true);
    if (r.ok) assert.equal(r.value.snooze_hint, null);
  });

  it('accepts every soft intent', () => {
    for (const intent of REPLY_INTENTS) {
      const text = JSON.stringify({
        intent,
        confidence: 0.5,
        reason: `picked ${intent}`,
        snooze_hint: intent === 'snooze' ? 'unspecified' : null
      });
      const r = validateReplyClassifierOutput(text);
      assert.equal(r.ok, true, `expected ok for intent=${intent}`);
    }
  });
});

describe('validateReplyClassifierOutput — normalization', () => {
  it('NORMALIZES non-snooze intent + non-null hint to null hint (orchestrator never reads it for non-snooze)', () => {
    const text = JSON.stringify({
      intent: 'ignore',
      confidence: 0.8,
      reason: 'x',
      snooze_hint: 'tomorrow' // model mistake — should be null
    });
    const r = validateReplyClassifierOutput(text);
    assert.equal(r.ok, true);
    if (r.ok) assert.equal(r.value.snooze_hint, null);
  });

  it('strips code-fence wrapping', () => {
    const inner = JSON.stringify({
      intent: 'why',
      confidence: 0.9,
      reason: 'asked',
      snooze_hint: null
    });
    const r = validateReplyClassifierOutput(`\`\`\`json\n${inner}\n\`\`\``);
    assert.equal(r.ok, true);
    if (r.ok) assert.equal(r.value.intent, 'why');
  });

  it('truncates an over-long reason (≤240 chars) rather than failing', () => {
    const longReason = 'A'.repeat(500);
    const text = JSON.stringify({
      intent: 'snooze',
      confidence: 0.9,
      reason: longReason,
      snooze_hint: 'later'
    });
    const r = validateReplyClassifierOutput(text);
    assert.equal(r.ok, true);
    if (r.ok) assert.ok(r.value.reason.length <= 240);
  });
});

describe('validateReplyClassifierOutput — failures', () => {
  it('rejects empty input', () => {
    const r = validateReplyClassifierOutput('');
    assert.equal(r.ok, false);
    if (!r.ok) assert.match(r.reason, /empty/);
  });

  it('rejects non-JSON', () => {
    const r = validateReplyClassifierOutput('not json');
    assert.equal(r.ok, false);
    if (!r.ok) assert.match(r.reason, /not JSON/);
  });

  it('rejects JSON array', () => {
    const r = validateReplyClassifierOutput('[]');
    assert.equal(r.ok, false);
  });

  it('rejects compliance intent attempt — STOP must not come from classifier', () => {
    const text = JSON.stringify({
      intent: 'stop',
      confidence: 1.0,
      reason: 'user said stop',
      snooze_hint: null
    });
    const r = validateReplyClassifierOutput(text);
    assert.equal(r.ok, false);
    if (!r.ok) assert.match(r.reason, /intent/);
  });

  it('rejects out-of-range confidence', () => {
    for (const c of [-0.1, 1.1, Number.NaN, Number.POSITIVE_INFINITY]) {
      const text = JSON.stringify({
        intent: 'snooze',
        confidence: c,
        reason: 'x',
        snooze_hint: 'later'
      });
      const r = validateReplyClassifierOutput(text);
      assert.equal(r.ok, false, `expected fail for confidence=${c}`);
    }
  });

  it('rejects non-number confidence', () => {
    const text = JSON.stringify({
      intent: 'snooze',
      confidence: 'high',
      reason: 'x',
      snooze_hint: 'later'
    });
    const r = validateReplyClassifierOutput(text);
    assert.equal(r.ok, false);
  });

  it('rejects unknown snooze_hint value', () => {
    const text = JSON.stringify({
      intent: 'snooze',
      confidence: 0.9,
      reason: 'x',
      snooze_hint: 'next_week'
    });
    const r = validateReplyClassifierOutput(text);
    assert.equal(r.ok, false);
    if (!r.ok) assert.match(r.reason, /snooze_hint/);
  });

  it('rejects non-string snooze_hint type', () => {
    const text = JSON.stringify({
      intent: 'snooze',
      confidence: 0.9,
      reason: 'x',
      snooze_hint: 5
    });
    const r = validateReplyClassifierOutput(text);
    assert.equal(r.ok, false);
  });

  it('rejects non-string reason', () => {
    const text = JSON.stringify({
      intent: 'why',
      confidence: 0.9,
      reason: 42,
      snooze_hint: null
    });
    const r = validateReplyClassifierOutput(text);
    assert.equal(r.ok, false);
  });
});

describe('validateReplyClassifierOutput — output immutability', () => {
  it('returned value is frozen', () => {
    const text = JSON.stringify({
      intent: 'snooze',
      confidence: 0.9,
      reason: 'x',
      snooze_hint: 'later'
    });
    const r = validateReplyClassifierOutput(text);
    assert.equal(r.ok, true);
    if (r.ok) {
      assert.throws(() => {
        (r.value as unknown as { intent: string }).intent = 'ignore';
      });
    }
  });
});

describe('SNOOZE_HINTS_NON_NULL', () => {
  it('declares exactly the four non-null hints', () => {
    assert.deepEqual([...SNOOZE_HINTS_NON_NULL].sort(), [
      'later',
      'remind_me_later',
      'tomorrow',
      'unspecified'
    ]);
  });
});
