import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { WithingsHealthInput, WithingsHealthOutput } from './types.js';

const WITHINGS_MEASURE_TYPE_REQUIRED = 'WITHINGS_MEASURE_TYPE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'withings-health',
  plane: 'hands',
  requiredScopes: ['user.metrics', 'user.activity'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['get_measurements', 'trend_summary'] },
      measure_type: {
        type: 'string',
        enum: ['weight', 'body_fat_pct', 'muscle_mass_kg', 'heart_rate_bpm']
      },
      start_date: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' },
      end_date: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'measure_type', 'measurements', 'trend', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['withings-health'] },
      action: { type: 'string', enum: ['get_measurements', 'trend_summary'] },
      measure_type: {
        type: 'string',
        enum: ['weight', 'body_fat_pct', 'muscle_mass_kg', 'heart_rate_bpm']
      },
      measurements: { type: 'array' },
      trend: { type: 'string', enum: ['up', 'down', 'stable'] },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'withings-health',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            WITHINGS_MEASURE_TYPE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as WithingsHealthInput);
      const output = OutputSchema.parse(data) as WithingsHealthOutput;
      return {
        skill_id: 'withings-health',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'withings-health',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'withings-health execution failed',
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
