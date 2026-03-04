import { z } from 'zod';

export const InputSchema = z.object({
  query: z.string().min(2).max(500),
  max_results: z.number().int().min(1).max(20).optional(),
  include_domains: z.array(z.string().min(3).max(255)).max(10).optional()
});

export const OutputSchema = z.object({
  provider: z.literal('tavily'),
  results: z.array(
    z.object({
      title: z.string(),
      url: z.string().url(),
      content: z.string(),
      score: z.number().min(0).max(1)
    })
  )
});
