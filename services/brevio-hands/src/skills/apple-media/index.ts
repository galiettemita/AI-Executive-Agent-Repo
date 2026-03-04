import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { AppleMediaInput, AppleMediaOutput } from './types.js';

const APPLE_MEDIA_DEVICE_REQUIRED = 'APPLE_MEDIA_DEVICE_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'apple-media',
  plane: 'hands',
  requiredScopes: ['media.control'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['discover_devices', 'playback_status', 'control_playback'] },
      device_name: { type: 'string', minLength: 2, maxLength: 120 },
      command: { type: 'string', enum: ['play', 'pause', 'next', 'previous', 'set_volume'] },
      volume_pct: { type: 'integer', minimum: 0, maximum: 100 },
      source: { type: 'string', minLength: 2, maxLength: 160 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'devices', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['apple-media'] },
      action: { type: 'string', enum: ['discover_devices', 'playback_status', 'control_playback'] },
      devices: { type: 'array' },
      now_playing: { type: 'object' },
      applied_command: { type: 'string', enum: ['play', 'pause', 'next', 'previous', 'set_volume'] },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'apple-media',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || APPLE_MEDIA_DEVICE_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as AppleMediaInput);
      const output = OutputSchema.parse(data) as AppleMediaOutput;
      return {
        skill_id: 'apple-media',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'apple-media',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'apple-media execution failed',
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
