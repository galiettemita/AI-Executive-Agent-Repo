import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { YoutubeSummarizerInput, YoutubeSummarizerOutput } from './types.js';

const YOUTUBE_SUMMARIZER_VIDEO_REQUIRED = 'YOUTUBE_SUMMARIZER_VIDEO_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'youtube-summarizer',
  plane: 'hands',
  requiredScopes: ['youtube.readonly'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['summarize_video', 'key_points'] },
      video_id: { type: 'string', minLength: 6, maxLength: 40 },
      video_url: { type: 'string', format: 'uri', pattern: '^https://' },
      max_points: { type: 'integer', minimum: 1, maximum: 20 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'video_id', 'summary', 'key_points', 'transcript_excerpt'],
    properties: {
      provider: { type: 'string', enum: ['youtube-summarizer'] },
      action: { type: 'string', enum: ['summarize_video', 'key_points'] },
      video_id: { type: 'string' },
      summary: { type: 'string' },
      key_points: { type: 'array' },
      transcript_excerpt: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'youtube-summarizer',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            YOUTUBE_SUMMARIZER_VIDEO_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as YoutubeSummarizerInput);
      const output = OutputSchema.parse(data) as YoutubeSummarizerOutput;
      return {
        skill_id: 'youtube-summarizer',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'youtube-summarizer',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'youtube-summarizer execution failed',
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
