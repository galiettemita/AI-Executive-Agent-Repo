import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { OverseerrInput, OverseerrOutput } from './types.js';

const OVERSEERR_QUERY_REQUIRED = 'OVERSEERR_QUERY_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'overseerr',
  plane: 'hands',
  requiredScopes: ['overseerr.request.write'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['search_media', 'request_media', 'list_requests'] },
      query: { type: 'string', minLength: 2, maxLength: 240 },
      media_type: { type: 'string', enum: ['movie', 'tv'] },
      media_id: { type: 'string', minLength: 2, maxLength: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'requests', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['overseerr'] },
      action: { type: 'string', enum: ['search_media', 'request_media', 'list_requests'] },
      requests: { type: 'array', items: { type: 'object' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'overseerr',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || OVERSEERR_QUERY_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as OverseerrInput);
      const output = OutputSchema.parse(data) as OverseerrOutput;
      return {
        skill_id: 'overseerr',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'overseerr',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'overseerr execution failed',
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
