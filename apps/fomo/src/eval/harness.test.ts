import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { MockModelBackend } from '../core/model-backends/mock.ts';
import { type EvalFixture, runEval } from './harness.ts';

type Label = 'important' | 'not_important';

// Parse '{"label":"important"}' → 'important'; return null on anything else.
function parseJsonLabel(text: string): Label | null {
  try {
    const parsed = JSON.parse(text) as unknown;
    if (typeof parsed !== 'object' || parsed === null) return null;
    const label = (parsed as Record<string, unknown>).label;
    if (label === 'important' || label === 'not_important') return label;
    return null;
  } catch {
    return null;
  }
}

const FIXTURES: readonly EvalFixture<Label>[] = Object.freeze([
  Object.freeze({ prompt: 'a', expected_label: 'important' as Label }),
  Object.freeze({ prompt: 'b', expected_label: 'important' as Label }),
  Object.freeze({ prompt: 'c', expected_label: 'not_important' as Label }),
  Object.freeze({ prompt: 'd', expected_label: 'not_important' as Label }),
  Object.freeze({ prompt: 'e', expected_label: 'not_important' as Label })
]);

describe('runEval — perfect classifier', () => {
  it('precision 1.0, recall 1.0, all parsed', async () => {
    const backend = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: {
        a: { text: '{"label":"important"}', input_tokens: 1, output_tokens: 1 },
        b: { text: '{"label":"important"}', input_tokens: 1, output_tokens: 1 },
        c: { text: '{"label":"not_important"}', input_tokens: 1, output_tokens: 1 },
        d: { text: '{"label":"not_important"}', input_tokens: 1, output_tokens: 1 },
        e: { text: '{"label":"not_important"}', input_tokens: 1, output_tokens: 1 }
      }
    });
    const result = await runEval({
      fixtures: FIXTURES,
      backend,
      parseLabel: parseJsonLabel,
      positiveLabel: 'important'
    });
    assert.equal(result.total, 5);
    assert.equal(result.json_valid, 5);
    assert.equal(result.actually_positive, 2);
    assert.equal(result.predicted_positive, 2);
    assert.equal(result.tp, 2);
    assert.equal(result.fp, 0);
    assert.equal(result.tn, 3);
    assert.equal(result.fn, 0);
    assert.equal(result.precision, 1.0);
    assert.equal(result.recall, 1.0);
  });
});

describe('runEval — over-eager classifier (false positives)', () => {
  it('predicts everything positive: precision 0.4, recall 1.0', async () => {
    const backend = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: {
        a: { text: '{"label":"important"}', input_tokens: 1, output_tokens: 1 },
        b: { text: '{"label":"important"}', input_tokens: 1, output_tokens: 1 },
        c: { text: '{"label":"important"}', input_tokens: 1, output_tokens: 1 },
        d: { text: '{"label":"important"}', input_tokens: 1, output_tokens: 1 },
        e: { text: '{"label":"important"}', input_tokens: 1, output_tokens: 1 }
      }
    });
    const result = await runEval({
      fixtures: FIXTURES,
      backend,
      parseLabel: parseJsonLabel,
      positiveLabel: 'important'
    });
    assert.equal(result.tp, 2);
    assert.equal(result.fp, 3);
    assert.equal(result.tn, 0);
    assert.equal(result.fn, 0);
    assert.equal(result.precision, 2 / 5);
    assert.equal(result.recall, 1.0);
  });
});

describe('runEval — under-eager classifier (false negatives)', () => {
  it('predicts everything negative: recall 0, precision null', async () => {
    const backend = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: {
        a: { text: '{"label":"not_important"}', input_tokens: 1, output_tokens: 1 },
        b: { text: '{"label":"not_important"}', input_tokens: 1, output_tokens: 1 },
        c: { text: '{"label":"not_important"}', input_tokens: 1, output_tokens: 1 },
        d: { text: '{"label":"not_important"}', input_tokens: 1, output_tokens: 1 },
        e: { text: '{"label":"not_important"}', input_tokens: 1, output_tokens: 1 }
      }
    });
    const result = await runEval({
      fixtures: FIXTURES,
      backend,
      parseLabel: parseJsonLabel,
      positiveLabel: 'important'
    });
    assert.equal(result.tp, 0);
    assert.equal(result.fp, 0);
    assert.equal(result.tn, 3);
    assert.equal(result.fn, 2);
    assert.equal(result.recall, 0);
    assert.equal(result.precision, null, 'precision is null when nothing positive was predicted');
  });
});

describe('runEval — null fixture set', () => {
  it('returns null precision and null recall when there are zero fixtures', async () => {
    const backend = new MockModelBackend({ model_name: 'mock-classifier-tiny' });
    const result = await runEval({
      fixtures: [],
      backend,
      parseLabel: parseJsonLabel,
      positiveLabel: 'important'
    });
    assert.equal(result.total, 0);
    assert.equal(result.precision, null);
    assert.equal(result.recall, null);
  });

  it('returns null recall when no fixture is actually_positive (cannot compute)', async () => {
    const backend = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      default: { text: '{"label":"not_important"}', input_tokens: 1, output_tokens: 1 }
    });
    const result = await runEval({
      fixtures: [
        { prompt: 'x', expected_label: 'not_important' as Label },
        { prompt: 'y', expected_label: 'not_important' as Label }
      ],
      backend,
      parseLabel: parseJsonLabel,
      positiveLabel: 'important'
    });
    assert.equal(result.actually_positive, 0);
    assert.equal(result.recall, null);
  });
});

describe('runEval — JSON-validity tracking', () => {
  it('counts unparseable outputs in json_valid only when parseLabel returns non-null', async () => {
    const backend = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: {
        a: { text: 'garbage not json', input_tokens: 1, output_tokens: 1 },
        b: { text: '{"label":"important"}', input_tokens: 1, output_tokens: 1 },
        c: { text: '{"wrong":"shape"}', input_tokens: 1, output_tokens: 1 },
        d: { text: '{"label":"not_important"}', input_tokens: 1, output_tokens: 1 },
        e: { text: 'plain text', input_tokens: 1, output_tokens: 1 }
      }
    });
    const result = await runEval({
      fixtures: FIXTURES,
      backend,
      parseLabel: parseJsonLabel,
      positiveLabel: 'important'
    });
    // Only b and d parse into valid Labels.
    assert.equal(result.json_valid, 2);
    assert.equal(result.predicted_positive, 1);
  });

  it('treats backend throws as not-parsed and not-predicted', async () => {
    const backend = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      responses: {
        a: { text: '{"label":"important"}', input_tokens: 1, output_tokens: 1 }
        // b, c, d, e have no canned response and no default → backend throws.
      }
    });
    const result = await runEval({
      fixtures: FIXTURES,
      backend,
      parseLabel: parseJsonLabel,
      positiveLabel: 'important'
    });
    assert.equal(result.json_valid, 1);
    assert.equal(result.predicted_positive, 1);
    assert.equal(result.tp, 1);
    assert.equal(result.fn, 1, '"b" (important) had no response → counted as fn');
  });
});

describe('runEval — result is frozen', () => {
  it('cannot mutate the returned result', async () => {
    const backend = new MockModelBackend({
      model_name: 'mock-classifier-tiny',
      default: { text: '{"label":"important"}', input_tokens: 1, output_tokens: 1 }
    });
    const result = await runEval({
      fixtures: [{ prompt: 'x', expected_label: 'important' as Label }],
      backend,
      parseLabel: parseJsonLabel,
      positiveLabel: 'important'
    });
    assert.throws(() => {
      (result as unknown as { tp: number }).tp = 999;
    });
  });
});
