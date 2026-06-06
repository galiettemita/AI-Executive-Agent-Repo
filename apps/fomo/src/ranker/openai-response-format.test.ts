// Phase v0.5.6 — ranker response_format schema regression guards.
//
// The schema constant is shared between the polling-worker bootstrap and
// the smoke-eval script; both send OpenAI the same contract. v0.5.6 added
// `maxLength: 180` on `reason` so OpenAI strict mode enforces it server-
// side. The client-side validator's MAX_REASON_LEN must stay in sync.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  RANKER_OPENAI_RESPONSE_FORMAT,
  RANKER_REASON_MAX_LEN
} from './openai-response-format.ts';

interface RankerSchemaShape {
  readonly type: 'object';
  readonly properties: {
    readonly label: { readonly type: 'string'; readonly enum: readonly string[] };
    readonly score: { readonly type: 'number' };
    readonly reason: { readonly type: 'string'; readonly maxLength?: number };
  };
  readonly required: readonly string[];
  readonly additionalProperties: boolean;
}

describe('RANKER_OPENAI_RESPONSE_FORMAT — v0.5.6 schema regression guards', () => {
  it('exports RANKER_REASON_MAX_LEN === 180', () => {
    assert.equal(RANKER_REASON_MAX_LEN, 180);
  });

  it('is strict-mode JSON schema', () => {
    assert.equal(RANKER_OPENAI_RESPONSE_FORMAT.type, 'json_schema');
    assert.equal(RANKER_OPENAI_RESPONSE_FORMAT.json_schema.strict, true);
    assert.equal(RANKER_OPENAI_RESPONSE_FORMAT.json_schema.name, 'ranker_decision');
  });

  it('reason property has maxLength === RANKER_REASON_MAX_LEN', () => {
    const schema = RANKER_OPENAI_RESPONSE_FORMAT.json_schema.schema as unknown as RankerSchemaShape;
    assert.equal(schema.properties.reason.type, 'string');
    assert.equal(schema.properties.reason.maxLength, RANKER_REASON_MAX_LEN);
    assert.equal(schema.properties.reason.maxLength, 180);
  });

  it('preserves prior-phase invariants: label enum + score type + required + additionalProperties=false', () => {
    const schema = RANKER_OPENAI_RESPONSE_FORMAT.json_schema.schema as unknown as RankerSchemaShape;
    assert.deepEqual(
      [...schema.properties.label.enum].sort(),
      ['important', 'not_important']
    );
    assert.equal(schema.properties.score.type, 'number');
    assert.deepEqual([...schema.required].sort(), ['label', 'reason', 'score']);
    assert.equal(schema.additionalProperties, false);
  });
});
