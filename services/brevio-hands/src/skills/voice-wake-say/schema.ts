import { z } from 'zod';

export const InputSchema = z
  .object({
    text: z.string().min(1).max(500),
    voice: z.string().min(2).max(40).optional(),
    rate_wpm: z.number().int().min(90).max(320).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('voice-wake-say'),
    voice: z.string().min(2).max(40),
    command: z.string().min(10).max(600),
    estimated_duration_ms: z.number().int().positive(),
    latency_budget_ms: z.literal(500)
  })
  .strict();
