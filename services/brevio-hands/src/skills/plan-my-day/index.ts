import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { PlanMyDayInput, PlanMyDayOutput } from './types.js';

const PLAN_MY_DAY_TASKS_REQUIRED = 'PLAN_MY_DAY_TASKS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'plan-my-day',
  plane: 'brain',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action', 'timezone', 'date'],
    properties: {
      action: { type: 'string', enum: ['build_plan', 'rebalance_plan'] },
      timezone: { type: 'string', minLength: 3, maxLength: 80 },
      date: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' },
      tasks: { type: 'array' },
      available_windows: { type: 'array' },
      disruptions: { type: 'array' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'time_blocks', 'overflow_tasks', 'strategy_notes'],
    properties: {
      provider: { type: 'string', enum: ['plan-my-day'] },
      action: { type: 'string', enum: ['build_plan', 'rebalance_plan'] },
      time_blocks: { type: 'array' },
      overflow_tasks: { type: 'array' },
      strategy_notes: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'plan-my-day',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || PLAN_MY_DAY_TASKS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as PlanMyDayInput);
      const output = OutputSchema.parse(data) as PlanMyDayOutput;
      return {
        skill_id: 'plan-my-day',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'plan-my-day',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'plan-my-day execution failed',
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
