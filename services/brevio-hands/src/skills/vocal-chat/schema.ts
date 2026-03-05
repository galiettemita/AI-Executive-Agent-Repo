import { z } from 'zod';

export const InputSchema = z
  .object({
    audio_url: z.string().url().startsWith('https://'),
    mime_type: z.enum(['audio/ogg', 'audio/mpeg', 'audio/wav', 'audio/mp4']),
    duration_ms: z.number().int().min(500).max(120000),
    response_voice: z.enum(['alloy', 'verse', 'sage']).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('vocal-chat'),
    transcript: z.string().min(1).max(4096),
    reply_text: z.string().min(1).max(4096),
    reply_audio_url: z.string().url().startsWith('https://'),
    stt_provider: z.enum(['asr', 'gemini-stt']),
    tts_provider: z.literal('openai-tts'),
    latency_budget_ms: z.literal(5000)
  })
  .strict();
