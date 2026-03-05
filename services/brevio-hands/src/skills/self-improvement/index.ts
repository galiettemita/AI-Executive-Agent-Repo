import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { SelfImprovementInput, SelfImprovementOutput } from './types.js';

const SELF_IMPROVEMENT_LESSON_REQUIRED = 'SELF_IMPROVEMENT_LESSON_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'self-improvement',
  plane: 'hands',
  requiredScopes: ['coaching.reflect'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['log_lesson', 'weekly_review'] },
      lesson: { type: 'string', minLength: 10, maxLength: 3000 },
      category: { type: 'string', minLength: 2, maxLength: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'improvements', 'next_steps', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['self-improvement'] },
      action: { type: 'string', enum: ['log_lesson', 'weekly_review'] },
      improvements: { type: 'array', items: { type: 'string' } },
      next_steps: { type: 'array', items: { type: 'string' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'self-improvement',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || SELF_IMPROVEMENT_LESSON_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as SelfImprovementInput);
      const output = OutputSchema.parse(data) as SelfImprovementOutput;
      return {
        skill_id: 'self-improvement',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'self-improvement',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'self-improvement execution failed',
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
