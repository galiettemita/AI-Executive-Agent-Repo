import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { SpotifyHistoryInput, SpotifyHistoryOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'spotify-history',
  plane: 'hands',
  requiredScopes: ['user-top-read', 'user-read-recently-played'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['top_tracks', 'top_artists', 'listening_summary'] },
      window: { type: 'string', enum: ['4w', '6m', '12m'] },
      limit: { type: 'integer', minimum: 1, maximum: 50 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'top_tracks', 'top_artists', 'total_listening_minutes', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['spotify-history'] },
      action: { type: 'string', enum: ['top_tracks', 'top_artists', 'listening_summary'] },
      top_tracks: { type: 'array' },
      top_artists: { type: 'array' },
      total_listening_minutes: { type: 'integer', minimum: 0 },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'spotify-history',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; '),
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as SpotifyHistoryInput);
      const output = OutputSchema.parse(data) as SpotifyHistoryOutput;
      return {
        skill_id: 'spotify-history',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'spotify-history',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'spotify-history execution failed',
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
