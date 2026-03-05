import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { AppleMailSearchInput, AppleMailSearchOutput } from './types.js';

const APPLE_MAIL_SEARCH_QUERY_REQUIRED = 'APPLE_MAIL_SEARCH_QUERY_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'apple-mail-search',
  plane: 'hands',
  requiredScopes: ['mail.read'],
  inputSchema: {
    type: 'object',
    required: ['action', 'query'],
    properties: {
      action: { type: 'string', enum: ['search_all', 'search_sender', 'search_subject'] },
      query: { type: 'string', minLength: 2, maxLength: 200 },
      mailbox: { type: 'string', minLength: 1, maxLength: 120 },
      limit: { type: 'integer', minimum: 1, maximum: 100 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'query', 'results', 'latency_profile_ms', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['apple-mail-search'] },
      action: { type: 'string', enum: ['search_all', 'search_sender', 'search_subject'] },
      query: { type: 'string' },
      results: { type: 'array' },
      latency_profile_ms: { type: 'integer', enum: [50] },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'apple-mail-search',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            APPLE_MAIL_SEARCH_QUERY_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as AppleMailSearchInput);
      const output = OutputSchema.parse(data) as AppleMailSearchOutput;
      return {
        skill_id: 'apple-mail-search',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'apple-mail-search',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'apple-mail-search execution failed',
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
