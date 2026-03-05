import { z } from 'zod';

export const InputSchema = z
  .object({
    prompt: z.string().min(3).max(1000),
    model: z.string().min(2).max(120).optional(),
    size: z.enum(['square', 'portrait', 'landscape']).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('fal-ai'),
    image_url: z.string().url(),
    model_used: z.string(),
    size: z.enum(['square', 'portrait', 'landscape'])
  })
  .strict();
