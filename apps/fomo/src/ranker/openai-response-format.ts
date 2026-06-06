// JSON Schema sent to OpenAI as response_format for the ranker. Forces
// server-side strict-output enforcement of { label, score, reason }.
// The client-side validator (validateRankerOutput) still runs as
// defense-in-depth: OpenAI strict mode does not accept `minimum`/`maximum`
// on numbers, so the 0..1 score bound is enforced client-side only.
//
// Shared between the 3C.2 smoke-eval script (apps/fomo/scripts/smoke-eval-3c2.ts)
// and the 3C.3 polling-worker bootstrap (apps/fomo/src/index.ts) so both
// paths send OpenAI the exact same contract.
//
// v0.5.6: added `maxLength: 180` on `reason` so OpenAI strict mode
// truncates server-side. The client-side validator's MAX_REASON_LEN
// must stay in sync with this value (apps/fomo/src/ranker/validator.ts).
// Founder-locked length policy 2026-06-05: reason budget ≤ 180 chars
// so the deterministic shell (sender + subject) can fit the rendered
// body inside the 220–280 char target / 320 hard cap.

import type { OpenAIResponseFormat } from '../core/model-backends/openai.js';

// Source of truth for the ranker reason maxLength. Mirrored by the
// validator's MAX_REASON_LEN. If you change one, change the other.
export const RANKER_REASON_MAX_LEN = 180;

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
        reason: { type: 'string', maxLength: RANKER_REASON_MAX_LEN }
      },
      required: ['label', 'score', 'reason'],
      additionalProperties: false
    })
  }
});
