import { z } from 'zod';

const ExaResultSchema = z.object({
  title: z.string(),
  url: z.string().url(),
  snippet: z.string(),
  score: z.number().min(0).max(1)
});

export const InputSchema = z
  .object({
    query: z.string().min(2).max(500),
    max_results: z.number().int().min(1).max(20).optional(),
    include_domains: z.array(z.string()).max(10).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('exa'),
    results: z.array(ExaResultSchema)
  })
  .strict();
