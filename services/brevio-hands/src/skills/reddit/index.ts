import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { RedditInput, RedditOutput } from './types.js';

const REDDIT_POST_CONFIRMATION_REQUIRED = 'REDDIT_POST_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'reddit',
  plane: 'hands',
  requiredScopes: ['read', 'submit', 'identity'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['search', 'list_hot', 'post'] },
      subreddit: { type: 'string', minLength: 2, maxLength: 64 },
      query: { type: 'string', minLength: 2, maxLength: 300 },
      title: { type: 'string', minLength: 1, maxLength: 300 },
      text: { type: 'string', minLength: 1, maxLength: 40000 },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['reddit'] },
      action: { type: 'string', enum: ['search', 'list_hot', 'post'] },
      posts: { type: 'array' },
      submitted: { type: 'boolean' },
      post_id: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'reddit',
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

    const request = parsed.data as RedditInput;
    if (request.action === 'post' && request.confirmed !== true) {
      return {
        skill_id: 'reddit',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: REDDIT_POST_CONFIRMATION_REQUIRED,
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
      const output = OutputSchema.parse(data) as RedditOutput;
      return {
        skill_id: 'reddit',
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
        skill_id: 'reddit',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'reddit execution failed',
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
