import { z } from 'zod';

const ItemSchema = z.object({
  source: z.string(),
  title: z.string(),
  url: z.string().url()
});

export const InputSchema = z
  .object({
    topic: z.string().min(2).max(200).optional(),
    max_items: z.number().int().min(1).max(20).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('news-aggregator'),
    items: z.array(ItemSchema)
  })
  .strict();
