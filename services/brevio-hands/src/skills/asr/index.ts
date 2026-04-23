import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { AsrInput, AsrOutput } from './types.js';

const ASR_AUDIO_URL_REQUIRED = 'ASR_AUDIO_URL_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'asr',
  plane: 'gateway',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['audio_url', 'mime_type', 'duration_ms'],
    properties: {
      audio_url: { type: 'string', format: 'uri', pattern: '^https://' },
      mime_type: { type: 'string', enum: ['audio/ogg', 'audio/mpeg', 'audio/wav', 'audio/mp4'] },
      duration_ms: { type: 'integer', minimum: 500, maximum: 120000 },
      language_hint: { type: 'string', minLength: 2, maxLength: 20 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'provider_mode', 'model', 'transcript', 'language', 'confidence', 'segments', 'word_timestamps', 'latency_budget_ms'],
    properties: {
      provider: { type: 'string', enum: ['asr'] },
      provider_mode: { type: 'string', enum: ['dev_mock', 'live'] },
      model: { type: 'string' },
      transcript: { type: 'string' },
      language: { type: 'string' },
      confidence: { type: 'number', minimum: 0, maximum: 1 },
      segments: { type: 'array' },
      word_timestamps: { type: 'array' },
      latency_budget_ms: { type: 'integer', enum: [3000] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'asr',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || ASR_AUDIO_URL_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as AsrInput);
      const output = OutputSchema.parse(data) as AsrOutput;
      return {
        skill_id: 'asr',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'asr',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'asr execution failed',
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
