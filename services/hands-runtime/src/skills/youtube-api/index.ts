import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { YouTubeInput, YouTubeOutput } from './types.js';

const adapter: ISkillAdapter = {
  id: 'youtube-api',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['mode'],
    properties: {
      mode: { type: 'string', enum: ['search', 'transcript', 'channel'] },
      query: { type: 'string' },
      video_id: { type: 'string' },
      channel_id: { type: 'string' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'mode'],
    properties: {
      provider: { type: 'string', enum: ['youtube'] },
      mode: { type: 'string', enum: ['search', 'transcript', 'channel'] },
      results: { type: 'array' },
      transcript: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'youtube-api',
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
      const data = await runClient(parsed.data as YouTubeInput);
      const output = OutputSchema.parse(data) as YouTubeOutput;
      return {
        skill_id: 'youtube-api',
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
      const missingVideoId = err instanceof Error && err.message === 'YOUTUBE_VIDEO_ID_REQUIRED';
      return {
        skill_id: 'youtube-api',
        status: 'FAILED',
        error: {
          code: missingVideoId ? 'VALIDATION_FAILED' : 'EXTERNAL_ERROR',
          message: missingVideoId
            ? 'video_id is required for transcript mode.'
            : err instanceof Error
              ? err.message
              : 'youtube-api execution failed',
          retryable: !missingVideoId,
          http_status: missingVideoId ? 400 : 502
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
