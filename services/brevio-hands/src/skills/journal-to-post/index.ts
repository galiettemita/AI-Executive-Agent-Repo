import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { JournalToPostInput, JournalToPostOutput } from './types.js';

const JOURNAL_TO_POST_ENTRY_REQUIRED = 'JOURNAL_TO_POST_ENTRY_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'journal-to-post',
  plane: 'hands',
  requiredScopes: ['social.draft'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['draft_post', 'generate_thread'] },
      journal_entry: { type: 'string', minLength: 20, maxLength: 10000 },
      platform: { type: 'string', enum: ['x', 'linkedin', 'bluesky'] }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'platform', 'post_text', 'thread_parts', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['journal-to-post'] },
      action: { type: 'string', enum: ['draft_post', 'generate_thread'] },
      platform: { type: 'string', enum: ['x', 'linkedin', 'bluesky'] },
      post_text: { type: 'string' },
      thread_parts: { type: 'array', items: { type: 'string' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'journal-to-post',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || JOURNAL_TO_POST_ENTRY_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as JournalToPostInput);
      const output = OutputSchema.parse(data) as JournalToPostOutput;
      return {
        skill_id: 'journal-to-post',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'journal-to-post',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'journal-to-post execution failed',
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
