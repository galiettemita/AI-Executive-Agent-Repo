import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ProsConsInput, ProsConsOutput } from './types.js';

const PROS_CONS_DECISION_FIELDS_REQUIRED = 'PROS_CONS_DECISION_FIELDS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'pros-cons',
  plane: 'hands',
  requiredScopes: ['decision.analysis'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['evaluate_decision'] },
      decision: { type: 'string', minLength: 5, maxLength: 500 },
      options: { type: 'array', items: { type: 'string' }, minItems: 2, maxItems: 10 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'options', 'recommendation', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['pros-cons'] },
      action: { type: 'string', enum: ['evaluate_decision'] },
      options: { type: 'array', items: { type: 'object' } },
      recommendation: { type: 'string' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'pros-cons',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || PROS_CONS_DECISION_FIELDS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ProsConsInput);
      const output = OutputSchema.parse(data) as ProsConsOutput;
      return {
        skill_id: 'pros-cons',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'pros-cons',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'pros-cons execution failed',
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
