import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { OpenAiTtsInput, OpenAiTtsOutput } from './types.js';

const OPENAI_TTS_TEXT_REQUIRED = 'OPENAI_TTS_TEXT_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'openai-tts',
  plane: 'gateway',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['text'],
    properties: {
      text: { type: 'string', minLength: 1, maxLength: 500 },
      voice: { type: 'string', enum: ['alloy', 'verse', 'sage'] },
      format: { type: 'string', enum: ['mp3', 'wav', 'ogg'] }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'voice', 'format', 'audio_url', 'estimated_duration_ms', 'latency_budget_ms'],
    properties: {
      provider: { type: 'string', enum: ['openai-tts'] },
      voice: { type: 'string', enum: ['alloy', 'verse', 'sage'] },
      format: { type: 'string', enum: ['mp3', 'wav', 'ogg'] },
      audio_url: { type: 'string', format: 'uri', pattern: '^https://' },
      estimated_duration_ms: { type: 'integer', minimum: 1 },
      latency_budget_ms: { type: 'integer', enum: [2000] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'openai-tts',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || OPENAI_TTS_TEXT_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as OpenAiTtsInput);
      const output = OutputSchema.parse(data) as OpenAiTtsOutput;
      return {
        skill_id: 'openai-tts',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'openai-tts',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'openai-tts execution failed',
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
