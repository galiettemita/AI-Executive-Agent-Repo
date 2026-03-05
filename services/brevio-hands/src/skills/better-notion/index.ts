import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { BetterNotionInput, BetterNotionOutput } from './types.js';

const BETTER_NOTION_CREATE_TITLE_REQUIRED = 'BETTER_NOTION_CREATE_TITLE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'better-notion',
  plane: 'hands',
  requiredScopes: ['read_content', 'update_content', 'insert_content'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['create_page', 'query_database', 'update_page'] },
      page_title: { type: 'string', minLength: 2, maxLength: 300 },
      page_id: { type: 'string', minLength: 2, maxLength: 120 },
      database_id: { type: 'string', minLength: 2, maxLength: 120 },
      content: { type: 'string', minLength: 2, maxLength: 20000 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'pages', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['better-notion'] },
      action: { type: 'string', enum: ['create_page', 'query_database', 'update_page'] },
      pages: { type: 'array', items: { type: 'object' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'better-notion',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || BETTER_NOTION_CREATE_TITLE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as BetterNotionInput);
      const output = OutputSchema.parse(data) as BetterNotionOutput;
      return {
        skill_id: 'better-notion',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'better-notion',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'better-notion execution failed',
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
