import { z } from 'zod';

const ResultSchema = z.object({
  title: z.string(),
  link: z.string().url(),
  source: z.string()
});

export const InputSchema = z
  .object({
    query: z.string().min(2).max(500),
    engine: z.enum(['google', 'amazon', 'yelp']).optional(),
    max_results: z.number().int().min(1).max(20).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('serpapi'),
    engine: z.enum(['google', 'amazon', 'yelp']),
    results: z.array(ResultSchema)
  })
  .strict();
