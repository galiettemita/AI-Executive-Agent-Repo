import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ApplePhotosInput, ApplePhotosOutput } from './types.js';

const APPLE_PHOTOS_QUERY_REQUIRED = 'APPLE_PHOTOS_QUERY_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'apple-photos',
  plane: 'hands',
  requiredScopes: ['photos.read'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['list_albums', 'search_photos', 'recent_photos'] },
      album_name: { type: 'string', minLength: 2, maxLength: 120 },
      query: { type: 'string', minLength: 2, maxLength: 120 },
      date_from: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' },
      date_to: { type: 'string', pattern: '^\\d{4}-\\d{2}-\\d{2}$' },
      limit: { type: 'integer', minimum: 1, maximum: 100 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'albums', 'photos', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['apple-photos'] },
      action: { type: 'string', enum: ['list_albums', 'search_photos', 'recent_photos'] },
      albums: { type: 'array' },
      photos: { type: 'array' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'apple-photos',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || APPLE_PHOTOS_QUERY_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ApplePhotosInput);
      const output = OutputSchema.parse(data) as ApplePhotosOutput;
      return {
        skill_id: 'apple-photos',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'apple-photos',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'apple-photos execution failed',
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
