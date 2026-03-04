import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { VoiceWakeSayInput, VoiceWakeSayOutput } from './types.js';

const VOICE_WAKE_SAY_TEXT_REQUIRED = 'VOICE_WAKE_SAY_TEXT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'voice-wake-say',
  plane: 'gateway',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['text'],
    properties: {
      text: { type: 'string', minLength: 1, maxLength: 500 },
      voice: { type: 'string', minLength: 2, maxLength: 40 },
      rate_wpm: { type: 'integer', minimum: 90, maximum: 320 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'voice', 'command', 'estimated_duration_ms', 'latency_budget_ms'],
    properties: {
      provider: { type: 'string', enum: ['voice-wake-say'] },
      voice: { type: 'string' },
      command: { type: 'string' },
      estimated_duration_ms: { type: 'integer', minimum: 1 },
      latency_budget_ms: { type: 'integer', enum: [500] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'voice-wake-say',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            VOICE_WAKE_SAY_TEXT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as VoiceWakeSayInput);
      const output = OutputSchema.parse(data) as VoiceWakeSayOutput;
      return {
        skill_id: 'voice-wake-say',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'voice-wake-say',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'voice-wake-say execution failed',
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
