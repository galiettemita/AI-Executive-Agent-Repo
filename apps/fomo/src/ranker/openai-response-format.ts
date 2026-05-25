// JSON Schema sent to OpenAI as response_format for the ranker. Forces
// server-side strict-output enforcement of { label, score, reason }.
// The client-side validator (validateRankerOutput) still runs as
// defense-in-depth: OpenAI strict mode does not accept `minimum`/`maximum`
// on numbers, so the 0..1 score bound is enforced client-side only.
//
// Shared between the 3C.2 smoke-eval script (apps/fomo/scripts/smoke-eval-3c2.ts)
// and the 3C.3 polling-worker bootstrap (apps/fomo/src/index.ts) so both
// paths send OpenAI the exact same contract.

import type { OpenAIResponseFormat } from '../core/model-backends/openai.js';

export const RANKER_OPENAI_RESPONSE_FORMAT: OpenAIResponseFormat = Object.freeze({
  type: 'json_schema',
  json_schema: {
    name: 'ranker_decision',
    strict: true,
    schema: Object.freeze({
      type: 'object',
      properties: {
        label: { type: 'string', enum: ['important', 'not_important'] },
        score: { type: 'number' },
        reason: { type: 'string' }
      },
      required: ['label', 'score', 'reason'],
      additionalProperties: false
    })
  }
});
