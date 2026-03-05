import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { NotionInput, NotionOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'notion',
  plane: 'hands',
  requiredScopes: ['read_content', 'update_content', 'insert_content'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['search', 'create_page', 'append_block'] },
      query: { type: 'string' },
      page_id: { type: 'string' },
      title: { type: 'string' },
      content: { type: 'string' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['notion'] },
      action: { type: 'string', enum: ['search', 'create_page', 'append_block'] },
      page_id: { type: 'string' },
      pages: { type: 'array' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'notion',
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
      const data = await runClient(parsed.data as NotionInput);
      const output = OutputSchema.parse(data) as NotionOutput;
      return {
        skill_id: 'notion',
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
      const validation =
        err instanceof Error &&
        (err.message === 'NOTION_TITLE_REQUIRED' || err.message === 'NOTION_APPEND_FIELDS_REQUIRED');

      return {
        skill_id: 'notion',
        status: 'FAILED',
        error: {
          code: validation ? 'VALIDATION_FAILED' : 'EXTERNAL_ERROR',
          message: validation
            ? 'Notion action requires required fields (title, page_id, content).'
            : err instanceof Error
              ? err.message
              : 'notion execution failed',
          retryable: !validation,
          http_status: validation ? 400 : 502
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
