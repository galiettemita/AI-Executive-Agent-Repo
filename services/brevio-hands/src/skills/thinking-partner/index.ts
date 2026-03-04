import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ThinkingPartnerInput, ThinkingPartnerOutput } from './types.js';

const THINKING_PARTNER_TOPIC_REQUIRED = 'THINKING_PARTNER_TOPIC_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'thinking-partner',
  plane: 'brain',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['clarify_problem', 'challenge_assumptions', 'decision_matrix'] },
      topic: { type: 'string', minLength: 5, maxLength: 240 },
      constraints: { type: 'array' },
      options: { type: 'array' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'reframed_problem', 'questions', 'assumptions_to_test'],
    properties: {
      provider: { type: 'string', enum: ['thinking-partner'] },
      action: { type: 'string', enum: ['clarify_problem', 'challenge_assumptions', 'decision_matrix'] },
      reframed_problem: { type: 'string' },
      questions: { type: 'array' },
      assumptions_to_test: { type: 'array' },
      decision_matrix: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'thinking-partner',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            THINKING_PARTNER_TOPIC_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ThinkingPartnerInput);
      const output = OutputSchema.parse(data) as ThinkingPartnerOutput;
      return {
        skill_id: 'thinking-partner',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'thinking-partner',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'thinking-partner execution failed',
          retryable: true,
          http_status: 502
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }
  },
  async healthCheck(): Promise<boolean> {
    return true;
  }
};

export default adapter;
