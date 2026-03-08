import { z } from 'zod';

export const InputSchema = z
  .object({
    query: z.string().min(1).max(200).optional(),
    genre: z.string().min(1).max(80).optional(),
    type: z.enum(['movie', 'tv']).optional()
  })
  .strict();

const ResultSchema = z.object({
  title: z.string(),
  year: z.number().int().min(1900).max(2100),
  rating: z.number().min(0).max(10),
  overview: z.string(),
  streaming: z.string()
});

export const OutputSchema = z
  .object({
    provider: z.literal('tmdb'),
    results: z.array(ResultSchema)
  })
  .strict();
