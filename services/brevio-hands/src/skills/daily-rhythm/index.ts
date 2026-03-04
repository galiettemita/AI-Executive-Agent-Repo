import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { DailyRhythmInput, DailyRhythmOutput } from './types.js';

const DAILY_RHYTHM_CONTEXT_REQUIRED = 'DAILY_RHYTHM_CONTEXT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'daily-rhythm',
  plane: 'brain',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action', 'timezone', 'date'],
    properties: {
      action: { type: 'string', enum: ['compose_briefing', 'wind_down_prompt'] },
      timezone: { type: 'string', minLength: 3, maxLength: 80 },
      date: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' },
      wake_time_local: { type: 'string', pattern: '^\\d{2}:\\d{2}$' },
      tasks: { type: 'array' },
      meetings: { type: 'array' },
      weather_summary: { type: 'string', minLength: 2, maxLength: 240 },
      energy_level: { type: 'string', enum: ['low', 'steady', 'high'] }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'briefing_text', 'priorities', 'schedule_blocks', 'nudges'],
    properties: {
      provider: { type: 'string', enum: ['daily-rhythm'] },
      action: { type: 'string', enum: ['compose_briefing', 'wind_down_prompt'] },
      briefing_text: { type: 'string' },
      priorities: { type: 'array' },
      schedule_blocks: { type: 'array' },
      nudges: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'daily-rhythm',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || DAILY_RHYTHM_CONTEXT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as DailyRhythmInput);
      const output = OutputSchema.parse(data) as DailyRhythmOutput;
      return {
        skill_id: 'daily-rhythm',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'daily-rhythm',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'daily-rhythm execution failed',
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
