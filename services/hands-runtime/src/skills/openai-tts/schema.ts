import { z } from 'zod';

export const InputSchema = z
  .object({
    text: z.string().min(1).max(500),
    voice: z.enum(['alloy', 'verse', 'sage']).optional(),
    format: z.enum(['mp3', 'wav', 'ogg']).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('openai-tts'),
    voice: z.enum(['alloy', 'verse', 'sage']),
    format: z.enum(['mp3', 'wav', 'ogg']),
    audio_url: z.string().url().startsWith('https://'),
    estimated_duration_ms: z.number().int().positive(),
    latency_budget_ms: z.literal(2000)
  })
  .strict();
