import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { SpotifyPlayerInput, SpotifyPlayerOutput } from './types.js';

const SPOTIFY_PLAYER_QUERY_REQUIRED = 'SPOTIFY_PLAYER_QUERY_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'spotify-player',
  plane: 'hands',
  requiredScopes: ['user-modify-playback-state', 'user-read-playback-state'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['search_tracks', 'queue_track', 'playback_status'] },
      query: { type: 'string', minLength: 2, maxLength: 200 },
      track_id: { type: 'string', minLength: 2, maxLength: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'tracks', 'queue_length', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['spotify-player'] },
      action: { type: 'string', enum: ['search_tracks', 'queue_track', 'playback_status'] },
      tracks: { type: 'array' },
      queue_length: { type: 'integer', minimum: 0 },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'spotify-player',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || SPOTIFY_PLAYER_QUERY_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as SpotifyPlayerInput);
      const output = OutputSchema.parse(data) as SpotifyPlayerOutput;
      return {
        skill_id: 'spotify-player',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'spotify-player',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'spotify-player execution failed',
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
