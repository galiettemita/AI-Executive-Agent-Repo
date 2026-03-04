import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ImapEmailInput, ImapEmailOutput } from './types.js';

const IMAP_EMAIL_CONFIRMATION_REQUIRED = 'IMAP_EMAIL_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'imap-email',
  plane: 'hands',
  requiredScopes: ['imap.read', 'imap.send'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['list', 'search', 'send'] },
      mailbox: { type: 'string', minLength: 2, maxLength: 120 },
      query: { type: 'string', minLength: 2, maxLength: 400 },
      to: { type: 'array' },
      subject: { type: 'string', minLength: 1, maxLength: 240 },
      body: { type: 'string', minLength: 1, maxLength: 4000 },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'mailbox'],
    properties: {
      provider: { type: 'string', enum: ['imap-email'] },
      action: { type: 'string', enum: ['list', 'search', 'send'] },
      mailbox: { type: 'string' },
      messages: { type: 'array' },
      sent: { type: 'boolean' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'imap-email',
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

    const request = parsed.data as ImapEmailInput;
    if (request.action === 'send' && request.confirmed !== true) {
      return {
        skill_id: 'imap-email',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: IMAP_EMAIL_CONFIRMATION_REQUIRED,
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
      const output = OutputSchema.parse(data) as ImapEmailOutput;
      return {
        skill_id: 'imap-email',
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
        skill_id: 'imap-email',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'imap-email execution failed',
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
