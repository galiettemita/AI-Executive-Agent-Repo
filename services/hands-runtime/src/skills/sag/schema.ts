import { z } from 'zod';

export const InputSchema = z
  .object({
    text: z.string().min(1).max(500),
    voice_id: z.string().min(2).max(80).optional(),
    style: z.enum(['neutral', 'warm', 'energetic']).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('sag'),
    voice_id: z.string().min(2).max(80),
    style: z.enum(['neutral', 'warm', 'energetic']),
    audio_url: z.string().url().startsWith('https://'),
    estimated_duration_ms: z.number().int().positive(),
    latency_budget_ms: z.literal(3000)
  })
  .strict();
