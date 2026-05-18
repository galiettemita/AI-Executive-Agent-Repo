import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type {
  VideoTranscriptDownloaderInput,
  VideoTranscriptDownloaderOutput
} from './types.js';

const VIDEO_TRANSCRIPT_VIDEO_REQUIRED = 'VIDEO_TRANSCRIPT_VIDEO_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'video-transcript-downloader',
  plane: 'hands',
  requiredScopes: ['youtube.readonly'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['fetch_transcript', 'fetch_subtitles'] },
      video_id: { type: 'string', minLength: 6, maxLength: 40 },
      video_url: { type: 'string', format: 'uri', pattern: '^https://' },
      language: { type: 'string', minLength: 2, maxLength: 10 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'video_id', 'language', 'transcript_text', 'segment_count'],
    properties: {
      provider: { type: 'string', enum: ['video-transcript-downloader'] },
      action: { type: 'string', enum: ['fetch_transcript', 'fetch_subtitles'] },
      video_id: { type: 'string' },
      language: { type: 'string' },
      transcript_text: { type: 'string' },
      segment_count: { type: 'integer', minimum: 1 },
      subtitle_url: { type: 'string', format: 'uri' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'video-transcript-downloader',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || VIDEO_TRANSCRIPT_VIDEO_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as VideoTranscriptDownloaderInput);
      const output = OutputSchema.parse(data) as VideoTranscriptDownloaderOutput;
      return {
        skill_id: 'video-transcript-downloader',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'video-transcript-downloader',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'video-transcript-downloader execution failed',
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
