// JSON Schema sent to OpenAI as response_format for the reply
// classifier. Forces server-side strict-output enforcement of the
// soft-intent set.
//
// Founder directive 2026-05-26:
//   * Compliance commands (STOP / UNSUBSCRIBE / CANCEL / START) are
//     handled by parseReplyDeterministic BEFORE this classifier runs.
//     The classifier never decides whether STOP means stop.
//   * Soft intents (snooze / ignore / ignore_sender / why /
//     false_positive / unclear) may use the LLM.
//   * Use strict JSON schema. Use egress-redacted reply context only.
//
// The client-side validator (validateReplyClassifierOutput) still
// runs as defense-in-depth: OpenAI strict mode does not accept
// `minimum`/`maximum` on numbers, so the 0..1 confidence bound is
// enforced client-side only.
//
// snooze_hint is OPTIONAL (`null` when intent !== 'snooze'). When
// intent === 'snooze' the classifier should pick one of three
// recognized hints; if none fit, 'unspecified' is the safe default.
// The calling orchestrator maps the hint to a concrete snooze_until
// timestamp.

import type { OpenAIResponseFormat } from '../core/model-backends/openai.js';

export const REPLY_PARSER_OPENAI_RESPONSE_FORMAT: OpenAIResponseFormat = Object.freeze({
  type: 'json_schema',
  json_schema: {
    name: 'reply_classification',
    strict: true,
    schema: Object.freeze({
      type: 'object',
      properties: {
        intent: {
          type: 'string',
          enum: [
            'snooze',
            'ignore',
            'ignore_sender',
            'why',
            'false_positive',
            'unclear'
          ]
        },
        confidence: { type: 'number' },
        reason: { type: 'string' },
        // OpenAI strict mode requires every property to appear in
        // `required`. Allowing null at the type level lets the model
        // omit a hint when intent !== 'snooze'.
        snooze_hint: {
          type: ['string', 'null'],
          enum: ['later', 'tomorrow', 'remind_me_later', 'unspecified', null]
        }
      },
      required: ['intent', 'confidence', 'reason', 'snooze_hint'],
      additionalProperties: false
    })
  }
});
