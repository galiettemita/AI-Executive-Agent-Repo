import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { OutlookInput, OutlookOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'outlook',
  plane: 'hands',
  requiredScopes: ['Mail.ReadWrite', 'Calendars.ReadWrite', 'offline_access'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['inbox_list', 'send', 'calendar_list'] },
      to: { type: 'array', items: { type: 'string', format: 'email' } },
      subject: { type: 'string' },
      body: { type: 'string' },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['outlook'] },
      action: { type: 'string', enum: ['inbox_list', 'send', 'calendar_list'] },
      confirmation_required: { type: 'boolean' },
      message_id: { type: 'string' },
      mails: { type: 'array' },
      events: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'outlook',
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
      const data = await runClient(parsed.data as OutlookInput);
      const output = OutputSchema.parse(data) as OutlookOutput;
      return {
        skill_id: 'outlook',
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
      const missingFields = err instanceof Error && err.message === 'OUTLOOK_SEND_FIELDS_REQUIRED';
      return {
        skill_id: 'outlook',
        status: 'FAILED',
        error: {
          code: missingFields ? 'VALIDATION_FAILED' : 'EXTERNAL_ERROR',
          message: missingFields
            ? 'send action requires to, subject, and body fields.'
            : err instanceof Error
              ? err.message
              : 'outlook execution failed',
          retryable: !missingFields,
          http_status: missingFields ? 400 : 502
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
