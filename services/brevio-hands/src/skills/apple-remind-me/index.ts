import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { AppleRemindMeInput, AppleRemindMeOutput } from './types.js';

const APPLE_REMIND_ME_TITLE_REQUIRED = 'APPLE_REMIND_ME_TITLE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'apple-remind-me',
  plane: 'hands',
  requiredScopes: ['apple.reminders.local'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['create', 'list', 'complete', 'delete'] },
      title: { type: 'string', minLength: 2, maxLength: 240 },
      due_at: { type: 'string', format: 'date-time' },
      reminder_id: { type: 'string', minLength: 3, maxLength: 120 },
      list: { type: 'string', minLength: 1, maxLength: 80 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'reminders', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['apple-reminders'] },
      action: { type: 'string', enum: ['create', 'list', 'complete', 'delete'] },
      reminders: { type: 'array', items: { type: 'object' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'apple-remind-me',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || APPLE_REMIND_ME_TITLE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as AppleRemindMeInput);
      const output = OutputSchema.parse(data) as AppleRemindMeOutput;
      return {
        skill_id: 'apple-remind-me',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'apple-remind-me',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'apple-remind-me execution failed',
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
