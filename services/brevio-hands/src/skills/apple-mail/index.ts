import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { AppleMailInput, AppleMailOutput } from './types.js';

const APPLE_MAIL_CONFIRMATION_REQUIRED = 'APPLE_MAIL_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'apple-mail',
  plane: 'hands',
  requiredScopes: ['apple.mail.read', 'apple.mail.send'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['list_inbox', 'search', 'send', 'reply'] },
      query: { type: 'string', minLength: 2, maxLength: 400 },
      to: { type: 'array' },
      subject: { type: 'string', minLength: 1, maxLength: 240 },
      body: { type: 'string', minLength: 1, maxLength: 4000 },
      reply_to_id: { type: 'string', minLength: 3, maxLength: 120 },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['apple-mail-local'] },
      action: { type: 'string', enum: ['list_inbox', 'search', 'send', 'reply'] },
      emails: { type: 'array' },
      sent: { type: 'boolean' },
      message_id: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'apple-mail',
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

    const request = parsed.data as AppleMailInput;
    if ((request.action === 'send' || request.action === 'reply') && request.confirmed !== true) {
      return {
        skill_id: 'apple-mail',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: APPLE_MAIL_CONFIRMATION_REQUIRED,
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
      const data = await runClient(request);
      const output = OutputSchema.parse(data) as AppleMailOutput;
      return {
        skill_id: 'apple-mail',
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
        skill_id: 'apple-mail',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'apple-mail execution failed',
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
