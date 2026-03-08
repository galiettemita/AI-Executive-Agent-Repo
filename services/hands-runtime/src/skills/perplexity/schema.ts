import { z } from 'zod';

const CitationSchema = z.object({
  title: z.string(),
  url: z.string().url()
});

export const InputSchema = z
  .object({
    query: z.string().min(2).max(800),
    model: z.string().min(2).max(120).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('perplexity'),
    answer: z.string().min(1),
    citations: z.array(CitationSchema).max(10)
  })
  .strict();
