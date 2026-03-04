import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { DexcomInput, DexcomOutput } from './types.js';

const DEXCOM_TIME_RANGE_REQUIRED = 'DEXCOM_TIME_RANGE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'dexcom',
  plane: 'hands',
  requiredScopes: ['egv:read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['glucose_readings', 'trend_alerts'] },
      start_time: { type: 'string', format: 'date-time' },
      end_time: { type: 'string', format: 'date-time' },
      minutes: { type: 'integer', minimum: 5, maximum: 1440 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'readings', 'alerts', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['dexcom'] },
      action: { type: 'string', enum: ['glucose_readings', 'trend_alerts'] },
      readings: { type: 'array' },
      alerts: { type: 'array' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'dexcom',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || DEXCOM_TIME_RANGE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as DexcomInput);
      const output = OutputSchema.parse(data) as DexcomOutput;
      return {
        skill_id: 'dexcom',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'dexcom',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'dexcom execution failed',
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
