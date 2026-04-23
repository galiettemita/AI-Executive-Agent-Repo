import { z } from 'zod';

export const InputSchema = z
  .object({
    text: z.string().min(1).max(500),
    voice: z.enum(['Alex', 'Samantha', 'Victoria', 'Daniel', 'Moira']).optional(),
    rate_wpm: z.number().int().min(90).max(320).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('voice-wake-say'),
    voice: z.enum(['Alex', 'Samantha', 'Victoria', 'Daniel', 'Moira']),
    command_argv: z.array(z.string()).min(7).max(7),
    estimated_duration_ms: z.number().int().positive(),
    latency_budget_ms: z.literal(500)
  })
  .strict();
