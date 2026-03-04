import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ClawdCoachInput, ClawdCoachOutput } from './types.js';

const CLAWD_COACH_GOAL_REQUIRED = 'CLAWD_COACH_GOAL_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'clawd-coach',
  plane: 'hands',
  requiredScopes: ['fitness.plan'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['build_plan', 'log_session'] },
      goal: { type: 'string', minLength: 2, maxLength: 240 },
      weeks: { type: 'integer', minimum: 1, maximum: 52 },
      session_notes: { type: 'string', minLength: 5, maxLength: 4000 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'workouts', 'milestones', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['clawd-coach'] },
      action: { type: 'string', enum: ['build_plan', 'log_session'] },
      workouts: { type: 'array', items: { type: 'string' } },
      milestones: { type: 'array', items: { type: 'string' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'clawd-coach',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || CLAWD_COACH_GOAL_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ClawdCoachInput);
      const output = OutputSchema.parse(data) as ClawdCoachOutput;
      return {
        skill_id: 'clawd-coach',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'clawd-coach',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'clawd-coach execution failed',
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
