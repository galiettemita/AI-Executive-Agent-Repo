import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { parseReplyDeterministic } from './deterministic.ts';

describe('parseReplyDeterministic — STOP-family (compliance, must NOT call LLM)', () => {
  for (const form of ['STOP', 'stop', 'Stop', 'sToP', '  stop  ', 'STOP!', 'stop.', 'stop?', 'stop,']) {
    it(`matches ${JSON.stringify(form)} as intent='stop' source='deterministic'`, () => {
      const r = parseReplyDeterministic(form);
      assert.ok(r);
      assert.equal(r?.intent, 'stop');
      assert.equal(r?.source, 'deterministic');
    });
  }

  for (const form of ['UNSUBSCRIBE', 'unsubscribe', 'Unsubscribe!', 'CANCEL', 'cancel', 'cancel.', 'END', 'end', 'QUIT', 'quit']) {
    it(`matches the compliance variant ${JSON.stringify(form)} as intent='stop'`, () => {
      const r = parseReplyDeterministic(form);
      assert.ok(r);
      assert.equal(r?.intent, 'stop');
      assert.equal(r?.source, 'deterministic');
    });
  }
});

describe('parseReplyDeterministic — START-family (resume compliance)', () => {
  for (const form of ['START', 'start', 'Start!', 'UNSTOP', 'unstop', 'RESUME', 'resume', 'resume.']) {
    it(`matches ${JSON.stringify(form)} as intent='start' source='deterministic'`, () => {
      const r = parseReplyDeterministic(form);
      assert.ok(r);
      assert.equal(r?.intent, 'start');
      assert.equal(r?.source, 'deterministic');
    });
  }
});

describe('parseReplyDeterministic — explicit non-matches (fall through to classifier)', () => {
  // These all CONTAIN compliance words but are not bare commands.
  // The classifier may or may not pick them up; the deterministic
  // pass MUST not match them (the founder directive: "Do not use
  // OpenAI to decide whether STOP means stop" — by symmetric logic,
  // "Can you stop sending these" is NOT a deterministic STOP).
  for (const phrase of [
    'Can you stop sending these',
    "I'd like to stop",
    'STOP please',
    'stop now',
    'stop later',
    'please cancel',
    'unsubscribe me',
    'I want to unsubscribe',
    'STOP STOP STOP',
    'cancel my subscription',
    'how do I cancel',
    // Phase v0.5.10 — removed 'why', 'ignore', 'not important' from this
    // list because the Q3.C explicit-feedback-phrase allowlist now
    // catches them deterministically (intent='why', 'ignore',
    // 'false_positive' respectively). The natural-language variations
    // that AREN'T canonical allowlist forms still fall through to the
    // classifier.
    'later',
    'tomorrow',
    '',
    '   ',
    '...',
    '!!!',
    'STOP and START'
  ]) {
    it(`does NOT match ${JSON.stringify(phrase)} (returns null; classifier will handle)`, () => {
      assert.equal(parseReplyDeterministic(phrase), null);
    });
  }
});

describe('parseReplyDeterministic — defensive against bad input types', () => {
  it('returns null for non-string input', () => {
    assert.equal(parseReplyDeterministic(null as unknown as string), null);
    assert.equal(parseReplyDeterministic(undefined as unknown as string), null);
    assert.equal(parseReplyDeterministic(42 as unknown as string), null);
    assert.equal(parseReplyDeterministic({} as unknown as string), null);
  });
});

describe('parseReplyDeterministic — output immutability', () => {
  it('returned match is frozen', () => {
    const r = parseReplyDeterministic('STOP');
    assert.ok(r);
    assert.throws(() => {
      (r as unknown as { intent: string }).intent = 'start';
    });
  });
});
