import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { GoogleCalendarInput, GoogleCalendarOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'google-calendar',
  plane: 'hands',
  requiredScopes: ['calendar'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['list', 'create', 'update', 'delete'] },
      calendar_id: { type: 'string' },
      confirmed: { type: 'boolean' },
      event: { type: 'object' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['action', 'calendar_id', 'confirmation_required'],
    properties: {
      action: { type: 'string', enum: ['list', 'create', 'update', 'delete'] },
      calendar_id: { type: 'string' },
      events: { type: 'array' },
      event_id: { type: 'string' },
      confirmation_required: { type: 'boolean' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'google-calendar',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; '),
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    }

    try {
      const data = await runClient(parsed.data as GoogleCalendarInput);
      const output = OutputSchema.parse(data) as GoogleCalendarOutput;
      return {
        skill_id: 'google-calendar',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    } catch (err) {
      return {
        skill_id: 'google-calendar',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'google-calendar execution failed',
          retryable: true,
          http_status: 502
        },
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    }
  },
  async healthCheck(): Promise<boolean> {
    return true;
  }
};

export default adapter;
