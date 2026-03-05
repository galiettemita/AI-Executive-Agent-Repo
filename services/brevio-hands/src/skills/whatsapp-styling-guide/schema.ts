import { z } from 'zod';

export const InputSchema = z
  .object({
    text: z.string().min(1).max(4096),
    style: z.enum(['default', 'bullet', 'numbered', 'emphasis']).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('whatsapp-styling-guide'),
    formatted_text: z.string().min(1).max(4096),
    applied_rules: z.array(z.string().min(2).max(120)).max(10),
    char_count: z.number().int().min(1).max(4096),
    latency_budget_ms: z.literal(10)
  })
  .strict();
