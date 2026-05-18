import { z } from 'zod';

const ResultSchema = z.object({
  name: z.string(),
  address: z.string(),
  category: z.string(),
  lat: z.number().min(-90).max(90),
  lng: z.number().min(-180).max(180)
});

export const InputSchema = z
  .object({
    query: z.string().min(2).max(500),
    area: z.string().min(2).max(120).optional(),
    grid_density: z.enum(['low', 'medium', 'high']).optional(),
    max_results: z.number().int().min(1).max(200).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.query.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'SPOTS_QUERY_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('spots'),
    grid_density: z.enum(['low', 'medium', 'high']),
    results: z.array(ResultSchema)
  })
  .strict();
