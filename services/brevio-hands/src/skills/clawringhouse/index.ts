import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ClawringhouseInput, ClawringhouseOutput } from './types.js';

const CLAWRINGHOUSE_ITEMS_REQUIRED = 'CLAWRINGHOUSE_ITEMS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'clawringhouse',
  plane: 'brain',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: {
        type: 'string',
        enum: ['detect_reorder_need', 'proactive_recommendations', 'schedule_reorder_reminder']
      },
      household_items: { type: 'array' },
      reminder_time_local: { type: 'string', pattern: '^\\d{2}:\\d{2}$' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'recommendations', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['clawringhouse'] },
      action: {
        type: 'string',
        enum: ['detect_reorder_need', 'proactive_recommendations', 'schedule_reorder_reminder']
      },
      recommendations: { type: 'array' },
      next_reminder_local: { type: 'string' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'clawringhouse',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || CLAWRINGHOUSE_ITEMS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ClawringhouseInput);
      const output = OutputSchema.parse(data) as ClawringhouseOutput;
      return {
        skill_id: 'clawringhouse',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'clawringhouse',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'clawringhouse execution failed',
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
