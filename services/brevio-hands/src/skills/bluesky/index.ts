import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { BlueskyInput, BlueskyOutput } from './types.js';

const BLUESKY_POST_CONFIRMATION_REQUIRED = 'BLUESKY_POST_CONFIRMATION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'bluesky',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['timeline', 'search', 'post'] },
      query: { type: 'string', minLength: 2, maxLength: 280 },
      text: { type: 'string', minLength: 1, maxLength: 300 },
      confirmed: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action'],
    properties: {
      provider: { type: 'string', enum: ['bluesky'] },
      action: { type: 'string', enum: ['timeline', 'search', 'post'] },
      posts: { type: 'array' },
      posted: { type: 'boolean' },
      uri: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'bluesky',
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

    const request = parsed.data as BlueskyInput;
    if (request.action === 'post' && request.confirmed !== true) {
      return {
        skill_id: 'bluesky',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: BLUESKY_POST_CONFIRMATION_REQUIRED,
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
      const output = OutputSchema.parse(data) as BlueskyOutput;
      return {
        skill_id: 'bluesky',
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
        skill_id: 'bluesky',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'bluesky execution failed',
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
