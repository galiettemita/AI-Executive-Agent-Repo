import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { VideoFramesInput, VideoFramesOutput } from './types.js';

const VIDEO_FRAMES_TIMESTAMP_REQUIRED = 'VIDEO_FRAMES_TIMESTAMP_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'video-frames',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action', 'video_url'],
    properties: {
      action: { type: 'string', enum: ['extract_frame', 'extract_frames'] },
      video_url: { type: 'string', format: 'uri', pattern: '^https://' },
      timestamp_seconds: { type: 'integer', minimum: 0, maximum: 86400 },
      frame_interval_seconds: { type: 'integer', minimum: 1, maximum: 3600 },
      frame_count: { type: 'integer', minimum: 1, maximum: 200 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'frame_urls', 'extracted_count', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['video-frames'] },
      action: { type: 'string', enum: ['extract_frame', 'extract_frames'] },
      frame_urls: { type: 'array' },
      extracted_count: { type: 'integer', minimum: 1 },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'video-frames',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            VIDEO_FRAMES_TIMESTAMP_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as VideoFramesInput);
      const output = OutputSchema.parse(data) as VideoFramesOutput;
      return {
        skill_id: 'video-frames',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'video-frames',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'video-frames execution failed',
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
