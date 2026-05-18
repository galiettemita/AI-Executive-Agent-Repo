import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { RadarrInput, RadarrOutput } from './types.js';

const RADARR_QUERY_REQUIRED = 'RADARR_QUERY_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'radarr',
  plane: 'hands',
  requiredScopes: ['radarr.movie.write'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['search_movie', 'add_movie', 'list_queue'] },
      query: { type: 'string', minLength: 2, maxLength: 240 },
      tmdb_id: { type: 'string', minLength: 2, maxLength: 120 },
      quality_profile: { type: 'string', minLength: 2, maxLength: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'movies', 'queue_count', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['radarr'] },
      action: { type: 'string', enum: ['search_movie', 'add_movie', 'list_queue'] },
      movies: { type: 'array', items: { type: 'object' } },
      queue_count: { type: 'integer', minimum: 0 },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'radarr',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || RADARR_QUERY_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as RadarrInput);
      const output = OutputSchema.parse(data) as RadarrOutput;
      return {
        skill_id: 'radarr',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'radarr',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'radarr execution failed',
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
