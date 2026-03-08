import { z } from 'zod';

const ResultSchema = z.object({
  place_id: z.string(),
  name: z.string(),
  formatted_address: z.string(),
  rating: z.number().min(0).max(5).optional(),
  open_now: z.boolean().optional()
});

export const InputSchema = z
  .object({
    query: z.string().min(2).max(500),
    location: z
      .object({
        lat: z.number().min(-90).max(90),
        lng: z.number().min(-180).max(180),
        radius_m: z.number().int().min(50).max(50000).optional()
      })
      .optional(),
    open_now: z.boolean().optional(),
    max_results: z.number().int().min(1).max(25).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.query.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'GOPLACES_QUERY_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('goplaces'),
    results: z.array(ResultSchema)
  })
  .strict();
