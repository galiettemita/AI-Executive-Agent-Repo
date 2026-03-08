import { z } from 'zod';

const ResultSchema = z.object({
  title: z.string(),
  url: z.string().url(),
  description: z.string()
});

export const InputSchema = z
  .object({
    query: z.string().min(2).max(500),
    max_results: z.number().int().min(1).max(20).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('brave-search'),
    results: z.array(ResultSchema)
  })
  .strict();
