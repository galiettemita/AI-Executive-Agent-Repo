import { z } from 'zod';

export const InputSchema = z.object({
  origin: z.string().min(2).max(200),
  destination: z.string().min(2).max(200),
  mode: z.enum(['driving', 'walking', 'bicycling', 'transit']).optional()
});

export const OutputSchema = z.object({
  distance_m: z.number().int().positive(),
  duration_s: z.number().int().positive(),
  mode: z.enum(['driving', 'walking', 'bicycling', 'transit']),
  steps: z.array(
    z.object({
      instruction: z.string(),
      distance_m: z.number().int().positive()
    })
  )
});
