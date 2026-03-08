import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ContentAdvisoryInput, ContentAdvisoryOutput } from './types.js';

const CONTENT_ADVISORY_TITLE_REQUIRED = 'CONTENT_ADVISORY_TITLE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'content-advisory',
  plane: 'hands',
  requiredScopes: ['content.rating.read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['evaluate_title'] },
      title: { type: 'string', minLength: 2, maxLength: 300 },
      media_type: { type: 'string', enum: ['movie', 'tv'] },
      age_target: { type: 'integer', minimum: 0, maximum: 99 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'categories', 'overall_advisory', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['content-advisory'] },
      action: { type: 'string', enum: ['evaluate_title'] },
      categories: { type: 'array', items: { type: 'object' } },
      overall_advisory: { type: 'string' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'content-advisory',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || CONTENT_ADVISORY_TITLE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ContentAdvisoryInput);
      const output = OutputSchema.parse(data) as ContentAdvisoryOutput;
      return {
        skill_id: 'content-advisory',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'content-advisory',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'content-advisory execution failed',
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
