import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { PostAtInput, PostAtOutput } from './types.js';

const POST_AT_TRACKING_REQUIRED = 'POST_AT_TRACKING_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'post-at',
  plane: 'hands',
  requiredScopes: ['parcel.read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['track_parcel'] },
      tracking_number: { type: 'string', minLength: 5, maxLength: 80 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'tracking_number', 'latest_status', 'checkpoints', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['post-at'] },
      action: { type: 'string', enum: ['track_parcel'] },
      tracking_number: { type: 'string' },
      latest_status: { type: 'string' },
      checkpoints: { type: 'array', items: { type: 'object' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'post-at',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || POST_AT_TRACKING_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as PostAtInput);
      const output = OutputSchema.parse(data) as PostAtOutput;
      return {
        skill_id: 'post-at',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'post-at',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'post-at execution failed',
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
