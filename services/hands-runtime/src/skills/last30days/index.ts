import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { Last30DaysInput, Last30DaysOutput } from './types.js';

const LAST30DAYS_QUERY_REQUIRED = 'LAST30DAYS_QUERY_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'last30days',
  plane: 'hands',
  requiredScopes: ['research.read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['scan_topic'] },
      query: { type: 'string', minLength: 2, maxLength: 500 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'highlights', 'sources', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['last30days'] },
      action: { type: 'string', enum: ['scan_topic'] },
      highlights: { type: 'array', items: { type: 'string' } },
      sources: { type: 'array', items: { type: 'string' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'last30days',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || LAST30DAYS_QUERY_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as Last30DaysInput);
      const output = OutputSchema.parse(data) as Last30DaysOutput;
      return {
        skill_id: 'last30days',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'last30days',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'last30days execution failed',
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
