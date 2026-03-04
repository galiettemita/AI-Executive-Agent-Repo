import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { CalctlInput, CalctlOutput } from './types.js';

const CALCTL_EVENT_FIELDS_REQUIRED = 'CALCTL_EVENT_FIELDS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'calctl',
  plane: 'hands',
  requiredScopes: ['apple.calendar.local'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['create_event', 'list_events', 'update_event', 'cancel_event'] },
      event_id: { type: 'string', minLength: 3, maxLength: 120 },
      title: { type: 'string', minLength: 2, maxLength: 240 },
      start_at: { type: 'string', format: 'date-time' },
      end_at: { type: 'string', format: 'date-time' },
      calendar: { type: 'string', minLength: 1, maxLength: 80 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'events', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['apple-calendar'] },
      action: { type: 'string', enum: ['create_event', 'list_events', 'update_event', 'cancel_event'] },
      events: { type: 'array', items: { type: 'object' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'calctl',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || CALCTL_EVENT_FIELDS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as CalctlInput);
      const output = OutputSchema.parse(data) as CalctlOutput;
      return {
        skill_id: 'calctl',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'calctl',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'calctl execution failed',
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
