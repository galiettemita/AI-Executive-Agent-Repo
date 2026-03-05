import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { MealPlannerInput, MealPlannerOutput } from './types.js';

const MEAL_PLANNER_HOUSEHOLD_SIZE_REQUIRED = 'MEAL_PLANNER_HOUSEHOLD_SIZE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'meal-planner',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['weekly_plan', 'grocery_rollup'] },
      household_size: { type: 'integer', minimum: 1, maximum: 20 },
      dietary_preferences: { type: 'array' },
      calorie_target_per_person: { type: 'integer', minimum: 1200, maximum: 5000 },
      meals_per_day: { type: 'integer', minimum: 1, maximum: 4 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'meals', 'grocery_items', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['meal-planner'] },
      action: { type: 'string', enum: ['weekly_plan', 'grocery_rollup'] },
      meals: { type: 'array' },
      grocery_items: { type: 'array' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'meal-planner',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            MEAL_PLANNER_HOUSEHOLD_SIZE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as MealPlannerInput);
      const output = OutputSchema.parse(data) as MealPlannerOutput;
      return {
        skill_id: 'meal-planner',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'meal-planner',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'meal-planner execution failed',
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
