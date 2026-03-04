import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { SleepCalculatorInput, SleepCalculatorOutput } from './types.js';

const SLEEP_CALCULATOR_WAKE_TIME_REQUIRED = 'SLEEP_CALCULATOR_WAKE_TIME_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'sleep-calculator',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['bedtime_from_wake', 'wake_from_bedtime'] },
      wake_time_local: { type: 'string', pattern: '^\\d{2}:\\d{2}$' },
      bedtime_local: { type: 'string', pattern: '^\\d{2}:\\d{2}$' },
      sleep_cycle_minutes: { type: 'integer', minimum: 60, maximum: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'recommendations', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['sleep-calculator'] },
      action: { type: 'string', enum: ['bedtime_from_wake', 'wake_from_bedtime'] },
      recommendations: { type: 'array' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'sleep-calculator',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            SLEEP_CALCULATOR_WAKE_TIME_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as SleepCalculatorInput);
      const output = OutputSchema.parse(data) as SleepCalculatorOutput;
      return {
        skill_id: 'sleep-calculator',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'sleep-calculator',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'sleep-calculator execution failed',
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
