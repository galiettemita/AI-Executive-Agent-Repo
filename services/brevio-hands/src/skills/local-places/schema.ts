import { z } from 'zod';

const ResultSchema = z.object({
  name: z.string(),
  address: z.string(),
  distance_km: z.number().nonnegative(),
  category: z.string()
});

export const InputSchema = z
  .object({
    query: z.string().min(2).max(500),
    near: z.string().min(2).max(120).optional(),
    radius_km: z.number().int().min(1).max(100).optional(),
    max_results: z.number().int().min(1).max(20).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.query.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'LOCAL_PLACES_QUERY_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('local-places'),
    results: z.array(ResultSchema)
  })
  .strict();
