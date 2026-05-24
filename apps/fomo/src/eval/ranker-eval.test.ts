import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { type ModelBackend } from '../core/model-router.ts';

import { RANKER_FIXTURES, buildRankerEvalFixtures, runRankerEval } from './ranker-eval.ts';

// A backend that responds with a fixed label and a valid JSON envelope.
function constantBackend(label: 'important' | 'not_important'): ModelBackend {
  return {
    name: () => 'constant-mock',
    call: async () => ({
      text: `{"label":"${label}","score":${label === 'important' ? 0.9 : 0.1},"reason":"constant"}`,
      input_tokens: 10,
      output_tokens: 6,
      model_name: 'constant-mock',
      latency_ms: 1
    })
  };
}

// A backend that returns the fixture's expected label by inspecting
// the prompt for the canonical expected-label hints we baked into the
// fixtures. We can't ACTUALLY look at expected_label from prompt text,
// so this backend is "perfect" only because we drive it differently
// via a lookup table in the test.
function perfectBackend(): ModelBackend {
  return {
    name: () => 'perfect-mock',
    call: async (req) => {
      // Decide the label from a signal we baked into the fixtures:
      // 'important' fixtures all carry an action-oriented keyword in
      // subject or body. We use very simple heuristics here just to
      // make this test backend deterministic — NOT a real classifier.
      const text = req.prompt.toLowerCase();
      const importantSignals = [
        'due tonight',
        'test results',
        'call me',
        'confirming your interview',
        'rent is due',
        'unusual sign-in',
        'submission window closes',
        'are you coming tonight',
        'flight tomorrow has been cancelled',
        'dismissal time change'
      ];
      const isImportant = importantSignals.some((s) => text.includes(s));
      const label = isImportant ? 'important' : 'not_important';
      return {
        text: `{"label":"${label}","score":${isImportant ? 0.9 : 0.1},"reason":"signal match"}`,
        input_tokens: 50,
        output_tokens: 8,
        model_name: 'perfect-mock',
        latency_ms: 1
      };
    }
  };
}

// A backend that emits malformed JSON for every fixture — all
// predictions count as schema_invalid (json_valid == 0).
function brokenBackend(): ModelBackend {
  return {
    name: () => 'broken-mock',
    call: async () => ({
      text: 'I think this is important',
      input_tokens: 10,
      output_tokens: 5,
      model_name: 'broken-mock',
      latency_ms: 1
    })
  };
}

describe('buildRankerEvalFixtures — egress + prompt build', () => {
  it('converts every RANKER_FIXTURES entry to an EvalFixture', () => {
    const converted = buildRankerEvalFixtures();
    assert.equal(converted.length, RANKER_FIXTURES.length);
    for (const c of converted) {
      // Prompt is non-empty and includes the subject from the source fixture.
      assert.ok(c.evalFixture.prompt.includes(c.fixture.subject));
      // expected_label round-trips.
      assert.equal(c.evalFixture.expected_label, c.fixture.expected_label);
    }
  });

  it('does NOT include leak-canary shapes (body_html / Received: headers / attachment filenames)', () => {
    const converted = buildRankerEvalFixtures();
    for (const c of converted) {
      assert.equal(c.evalFixture.prompt.includes('<html'), false);
      assert.equal(c.evalFixture.prompt.includes('Received: from'), false);
      assert.equal(c.evalFixture.prompt.includes('fixture-attachment.pdf'), false);
    }
  });

  it('returned list and its entries are frozen', () => {
    const converted = buildRankerEvalFixtures();
    assert.throws(() => {
      (converted as unknown as { length: number }).length = 0;
    });
    assert.throws(() => {
      (converted[0]!.evalFixture as unknown as { prompt: string }).prompt = 'mutated';
    });
  });
});

describe('runRankerEval — backend always returns "important"', () => {
  it('produces predicted_positive=total, recall=1.0, precision = importantFixtures/total', async () => {
    const r = await runRankerEval({ backend: constantBackend('important') });
    const expectedImportant = RANKER_FIXTURES.filter((f) => f.expected_label === 'important').length;
    assert.equal(r.summary.total, RANKER_FIXTURES.length);
    assert.equal(r.summary.predicted_positive, RANKER_FIXTURES.length);
    assert.equal(r.summary.actually_positive, expectedImportant);
    assert.equal(r.summary.tp, expectedImportant);
    assert.equal(r.summary.fp, RANKER_FIXTURES.length - expectedImportant);
    assert.equal(r.summary.recall, 1.0);
    assert.equal(r.summary.precision, expectedImportant / RANKER_FIXTURES.length);
  });
});

describe('runRankerEval — backend always returns "not_important"', () => {
  it('produces predicted_positive=0, precision=null (denominator zero), recall=0', async () => {
    const r = await runRankerEval({ backend: constantBackend('not_important') });
    const expectedImportant = RANKER_FIXTURES.filter((f) => f.expected_label === 'important').length;
    assert.equal(r.summary.predicted_positive, 0);
    assert.equal(r.summary.tp, 0);
    assert.equal(r.summary.fn, expectedImportant);
    assert.equal(r.summary.precision, null);
    assert.equal(r.summary.recall, 0);
  });
});

describe('runRankerEval — broken backend (malformed JSON)', () => {
  it('json_valid=0, all fixtures count as predicted_negative', async () => {
    const r = await runRankerEval({ backend: brokenBackend() });
    assert.equal(r.summary.json_valid, 0);
    assert.equal(r.summary.predicted_positive, 0);
    assert.equal(r.summary.tp, 0);
  });
});

describe('runRankerEval — perfect-mock backend (signal-match heuristic)', () => {
  it('achieves recall=1.0 and precision=1.0 on the synthetic fixture set', async () => {
    // This is a smoke check that the fixture set is internally
    // consistent: every 'important' fixture contains one of the
    // signal keywords that perfectBackend() looks for. If a future
    // fixture is added without a recognizable signal, this test
    // will fail and point you at the fixture set, not the harness.
    const r = await runRankerEval({ backend: perfectBackend() });
    assert.equal(r.summary.json_valid, RANKER_FIXTURES.length);
    assert.equal(r.summary.precision, 1.0);
    assert.equal(r.summary.recall, 1.0);
  });
});

describe('runRankerEval — coverage sanity', () => {
  it('fixture set has both labels represented (no degenerate single-class set)', () => {
    const importantCount = RANKER_FIXTURES.filter((f) => f.expected_label === 'important').length;
    const notImportantCount = RANKER_FIXTURES.filter(
      (f) => f.expected_label === 'not_important'
    ).length;
    assert.ok(importantCount >= 5, `expected >=5 important fixtures, got ${importantCount}`);
    assert.ok(
      notImportantCount >= 5,
      `expected >=5 not_important fixtures, got ${notImportantCount}`
    );
    assert.equal(importantCount + notImportantCount, RANKER_FIXTURES.length);
  });

  it('fixture ids are unique', () => {
    const ids = RANKER_FIXTURES.map((f) => f.id);
    assert.equal(new Set(ids).size, ids.length);
  });
});
