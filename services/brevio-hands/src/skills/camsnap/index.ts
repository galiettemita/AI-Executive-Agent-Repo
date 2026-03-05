import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { CamsnapInput, CamsnapOutput } from './types.js';

const CAMSNAP_CAMERA_REQUIRED = 'CAMSNAP_CAMERA_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'camsnap',
  plane: 'hands',
  requiredScopes: ['camera.capture'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['capture_frame', 'capture_clip'] },
      camera_id: { type: 'string', minLength: 2, maxLength: 120 },
      duration_seconds: { type: 'integer', minimum: 1, maximum: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'media_url', 'captured_at_utc', 'resolution', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['camsnap'] },
      action: { type: 'string', enum: ['capture_frame', 'capture_clip'] },
      media_url: { type: 'string' },
      captured_at_utc: { type: 'string' },
      resolution: { type: 'string' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'camsnap',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || CAMSNAP_CAMERA_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as CamsnapInput);
      const output = OutputSchema.parse(data) as CamsnapOutput;
      return {
        skill_id: 'camsnap',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'camsnap',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'camsnap execution failed',
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
