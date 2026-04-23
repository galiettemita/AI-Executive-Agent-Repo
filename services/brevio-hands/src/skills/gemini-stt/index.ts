import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { GeminiSttInput, GeminiSttOutput } from './types.js';

const GEMINI_STT_AUDIO_URL_REQUIRED = 'GEMINI_STT_AUDIO_URL_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'gemini-stt',
  plane: 'gateway',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['audio_url', 'duration_ms'],
    properties: {
      audio_url: { type: 'string', format: 'uri', pattern: '^https://' },
      duration_ms: { type: 'integer', minimum: 500, maximum: 120000 },
      language_hint: { type: 'string', minLength: 2, maxLength: 20 },
      include_speaker_labels: { type: 'boolean' }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'provider_mode', 'model', 'transcript', 'language', 'confidence', 'speakers', 'latency_budget_ms'],
    properties: {
      provider: { type: 'string', enum: ['gemini-stt'] },
      provider_mode: { type: 'string', enum: ['dev_mock', 'live'] },
      model: { type: 'string' },
      transcript: { type: 'string' },
      language: { type: 'string' },
      confidence: { type: 'number', minimum: 0, maximum: 1 },
      speakers: { type: 'array' },
      latency_budget_ms: { type: 'integer', enum: [5000] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'gemini-stt',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') ||
            GEMINI_STT_AUDIO_URL_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as GeminiSttInput);
      const output = OutputSchema.parse(data) as GeminiSttOutput;
      return {
        skill_id: 'gemini-stt',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'gemini-stt',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'gemini-stt execution failed',
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
