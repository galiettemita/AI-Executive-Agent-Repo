import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { validateRankerOutput } from './validator.ts';

describe('validateRankerOutput — happy path', () => {
  it('parses a clean single-line JSON object', () => {
    const r = validateRankerOutput('{"label":"important","score":0.92,"reason":"counselor reminder"}');
    assert.ok(r.ok);
    if (r.ok) {
      assert.equal(r.value.label, 'important');
      assert.equal(r.value.score, 0.92);
      assert.equal(r.value.reason, 'counselor reminder');
    }
  });

  it('accepts label="not_important"', () => {
    const r = validateRankerOutput('{"label":"not_important","score":0.05,"reason":"newsletter digest"}');
    assert.ok(r.ok);
    if (r.ok) assert.equal(r.value.label, 'not_important');
  });

  it('accepts score=0 and score=1 (inclusive bounds)', () => {
    const r0 = validateRankerOutput('{"label":"not_important","score":0,"reason":"x"}');
    assert.ok(r0.ok);
    const r1 = validateRankerOutput('{"label":"important","score":1,"reason":"x"}');
    assert.ok(r1.ok);
  });

  it('strips ```json code fence wrapper', () => {
    const r = validateRankerOutput('```json\n{"label":"important","score":0.7,"reason":"x"}\n```');
    assert.ok(r.ok);
    if (r.ok) assert.equal(r.value.label, 'important');
  });

  it('strips bare ``` code fence wrapper', () => {
    const r = validateRankerOutput('```\n{"label":"important","score":0.7,"reason":"x"}\n```');
    assert.ok(r.ok);
  });

  it('v0.5.6: fails-closed on overly long reason (no ellipsis truncation)', () => {
    // v0.5.6 founder Q4 lock 2026-06-05: "no arbitrary ellipsis from
    // truncation". Replaced the previous truncate-with-ellipsis behavior
    // with a fail-closed length check. OpenAI strict mode enforces
    // maxLength=180 server-side (openai-response-format.ts); this is
    // defense-in-depth.
    const longReason = 'a'.repeat(500);
    const r = validateRankerOutput(`{"label":"important","score":0.7,"reason":${JSON.stringify(longReason)}}`);
    assert.equal(r.ok, false);
    if (!r.ok) {
      assert.match(r.reason, /reason length 500 exceeds MAX_REASON_LEN=180/);
    }
  });

  it('v0.5.6: accepts reason at boundary (length === 180)', () => {
    const atBoundary = 'b'.repeat(180);
    const r = validateRankerOutput(`{"label":"important","score":0.7,"reason":${JSON.stringify(atBoundary)}}`);
    assert.ok(r.ok);
    if (r.ok) {
      assert.equal(r.value.reason.length, 180);
      // Critical: no ellipsis appended.
      assert.ok(!r.value.reason.endsWith('…'));
      assert.equal(r.value.reason, atBoundary);
    }
  });

  it('v0.5.6: fails-closed at length 181 (off-by-one regression guard)', () => {
    const tooLong = 'c'.repeat(181);
    const r = validateRankerOutput(`{"label":"important","score":0.7,"reason":${JSON.stringify(tooLong)}}`);
    assert.equal(r.ok, false);
  });

  it('returned decision object is frozen', () => {
    const r = validateRankerOutput('{"label":"important","score":0.5,"reason":"x"}');
    assert.ok(r.ok);
    if (r.ok) {
      assert.throws(() => {
        (r.value as unknown as { label: string }).label = 'mutated';
      });
    }
  });
});

describe('validateRankerOutput — fail-closed paths', () => {
  it('rejects empty string', () => {
    const r = validateRankerOutput('');
    assert.equal(r.ok, false);
    if (!r.ok) assert.match(r.reason, /empty/);
  });

  it('rejects whitespace-only output', () => {
    const r = validateRankerOutput('   \n  \t  ');
    assert.equal(r.ok, false);
    if (!r.ok) assert.match(r.reason, /empty/);
  });

  it('rejects non-JSON text', () => {
    const r = validateRankerOutput('this is not json');
    assert.equal(r.ok, false);
    if (!r.ok) assert.match(r.reason, /not JSON/);
  });

  it('rejects JSON array', () => {
    const r = validateRankerOutput('[1,2,3]');
    assert.equal(r.ok, false);
    if (!r.ok) assert.match(r.reason, /not a JSON object/);
  });

  it('rejects JSON null', () => {
    const r = validateRankerOutput('null');
    assert.equal(r.ok, false);
  });

  it('rejects JSON primitive (string)', () => {
    const r = validateRankerOutput('"important"');
    assert.equal(r.ok, false);
  });

  it('rejects missing label', () => {
    const r = validateRankerOutput('{"score":0.5,"reason":"x"}');
    assert.equal(r.ok, false);
    if (!r.ok) assert.match(r.reason, /label must be/);
  });

  it('rejects unknown label value', () => {
    const r = validateRankerOutput('{"label":"maybe","score":0.5,"reason":"x"}');
    assert.equal(r.ok, false);
    if (!r.ok) assert.match(r.reason, /label must be/);
  });

  it('rejects missing score', () => {
    const r = validateRankerOutput('{"label":"important","reason":"x"}');
    assert.equal(r.ok, false);
    if (!r.ok) assert.match(r.reason, /score must be a number/);
  });

  it('rejects non-numeric score', () => {
    const r = validateRankerOutput('{"label":"important","score":"high","reason":"x"}');
    assert.equal(r.ok, false);
  });

  it('rejects score < 0', () => {
    const r = validateRankerOutput('{"label":"important","score":-0.1,"reason":"x"}');
    assert.equal(r.ok, false);
  });

  it('rejects score > 1', () => {
    const r = validateRankerOutput('{"label":"important","score":1.1,"reason":"x"}');
    assert.equal(r.ok, false);
  });

  it('rejects NaN score', () => {
    // NaN is not representable in JSON literally; simulate via a backend
    // that returned a numeric string the JSON parser would accept as a
    // bad type later. JSON.parse won't accept literal NaN, so this is
    // effectively covered by "score not a number".
    const r = validateRankerOutput('{"label":"important","score":null,"reason":"x"}');
    assert.equal(r.ok, false);
  });

  it('rejects missing reason', () => {
    const r = validateRankerOutput('{"label":"important","score":0.5}');
    assert.equal(r.ok, false);
    if (!r.ok) assert.match(r.reason, /reason must be a string/);
  });

  it('rejects non-string reason', () => {
    const r = validateRankerOutput('{"label":"important","score":0.5,"reason":42}');
    assert.equal(r.ok, false);
  });
});
