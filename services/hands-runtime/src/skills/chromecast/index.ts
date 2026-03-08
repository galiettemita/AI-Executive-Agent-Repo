import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ChromecastInput, ChromecastOutput } from './types.js';

const CHROMECAST_CAST_FIELDS_REQUIRED = 'CHROMECAST_CAST_FIELDS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'chromecast',
  plane: 'hands',
  requiredScopes: ['chromecast.local.control'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['discover_devices', 'cast_media', 'pause', 'resume', 'stop', 'status'] },
      device_name: { type: 'string', minLength: 2, maxLength: 120 },
      media_url: { type: 'string', format: 'uri' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'devices', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['chromecast'] },
      action: { type: 'string', enum: ['discover_devices', 'cast_media', 'pause', 'resume', 'stop', 'status'] },
      devices: { type: 'array', items: { type: 'object' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'chromecast',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || CHROMECAST_CAST_FIELDS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ChromecastInput);
      const output = OutputSchema.parse(data) as ChromecastOutput;
      return {
        skill_id: 'chromecast',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'chromecast',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'chromecast execution failed',
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
