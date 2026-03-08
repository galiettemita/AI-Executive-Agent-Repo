import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { GoogleWorkspaceInput, GoogleWorkspaceOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'google-workspace',
  plane: 'hands',
  requiredScopes: ['gmail.modify', 'calendar', 'drive'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['gmail_list', 'gmail_send', 'calendar_list', 'drive_search'] },
      query: { type: 'string' },
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
      provider: { type: 'string', enum: ['google-workspace'] },
      action: { type: 'string', enum: ['gmail_list', 'gmail_send', 'calendar_list', 'drive_search'] },
      confirmation_required: { type: 'boolean' },
      message_id: { type: 'string' },
      mails: { type: 'array' },
      events: { type: 'array' },
      files: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'google-workspace',
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
      const data = await runClient(parsed.data as GoogleWorkspaceInput);
      const output = OutputSchema.parse(data) as GoogleWorkspaceOutput;
      return {
        skill_id: 'google-workspace',
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
      const missingFields =
        err instanceof Error && err.message === 'GOOGLE_WORKSPACE_SEND_FIELDS_REQUIRED';

      return {
        skill_id: 'google-workspace',
        status: 'FAILED',
        error: {
          code: missingFields ? 'VALIDATION_FAILED' : 'EXTERNAL_ERROR',
          message: missingFields
            ? 'gmail_send requires to, subject, and body fields.'
            : err instanceof Error
              ? err.message
              : 'google-workspace execution failed',
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
