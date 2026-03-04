import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { SonoscliInput, SonoscliOutput } from './types.js';

const SONOSCLI_PLAY_FIELDS_REQUIRED = 'SONOSCLI_PLAY_FIELDS_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'sonoscli',
  plane: 'hands',
  requiredScopes: ['sonos.control'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['discover', 'play', 'pause', 'set_volume', 'group', 'status'] },
      speaker_id: { type: 'string', minLength: 2, maxLength: 120 },
      query: { type: 'string', minLength: 2, maxLength: 240 },
      volume_pct: { type: 'integer', minimum: 0, maximum: 100 },
      group_with: { type: 'string', minLength: 2, maxLength: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'zones', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['sonoscli'] },
      action: { type: 'string', enum: ['discover', 'play', 'pause', 'set_volume', 'group', 'status'] },
      zones: { type: 'array', items: { type: 'object' } },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'sonoscli',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || SONOSCLI_PLAY_FIELDS_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as SonoscliInput);
      const output = OutputSchema.parse(data) as SonoscliOutput;
      return {
        skill_id: 'sonoscli',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'sonoscli',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'sonoscli execution failed',
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
