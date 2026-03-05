import { z } from 'zod';

export const InputSchema = z.object({
  query: z.string().min(2).max(200),
  max_price: z.number().positive().max(50000).optional(),
  category: z.string().min(2).max(100).optional(),
  limit: z.number().int().min(1).max(20).optional()
});

export const OutputSchema = z.object({
  provider: z.literal('mock_catalog'),
  results: z.array(
    z.object({
      title: z.string(),
      price: z.number(),
      url: z.string().url(),
      rating: z.number(),
      store: z.string()
    })
  )
});
