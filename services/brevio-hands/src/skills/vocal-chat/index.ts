import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { VocalChatInput, VocalChatOutput } from './types.js';

const VOCAL_CHAT_AUDIO_REQUIRED = 'VOCAL_CHAT_AUDIO_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'vocal-chat',
  plane: 'gateway',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['audio_url', 'mime_type', 'duration_ms'],
    properties: {
      audio_url: { type: 'string', format: 'uri', pattern: '^https://' },
      mime_type: { type: 'string', enum: ['audio/ogg', 'audio/mpeg', 'audio/wav', 'audio/mp4'] },
      duration_ms: { type: 'integer', minimum: 500, maximum: 120000 },
      response_voice: { type: 'string', enum: ['alloy', 'verse', 'sage'] }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: [
      'provider',
      'transcript',
      'reply_text',
      'reply_audio_url',
      'stt_provider',
      'tts_provider',
      'latency_budget_ms'
    ],
    properties: {
      provider: { type: 'string', enum: ['vocal-chat'] },
      transcript: { type: 'string' },
      reply_text: { type: 'string' },
      reply_audio_url: { type: 'string', format: 'uri', pattern: '^https://' },
      stt_provider: { type: 'string', enum: ['asr', 'gemini-stt'] },
      tts_provider: { type: 'string', enum: ['openai-tts'] },
      latency_budget_ms: { type: 'integer', enum: [5000] }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'vocal-chat',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message: parsed.error.issues.map((issue) => issue.message).join('; ') || VOCAL_CHAT_AUDIO_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as VocalChatInput);
      const output = OutputSchema.parse(data) as VocalChatOutput;
      return {
        skill_id: 'vocal-chat',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'vocal-chat',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'vocal-chat execution failed',
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
