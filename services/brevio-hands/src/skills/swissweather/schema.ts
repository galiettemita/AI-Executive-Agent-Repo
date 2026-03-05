import { z } from 'zod';

const ActionSchema = z.enum(['forecast']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    location: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.location) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SWISSWEATHER_LOCATION_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('swissweather'),
    action: ActionSchema,
    forecasts: z.array(
      z
        .object({
          day: z.string().min(2).max(40),
          condition: z.string().min(2).max(120),
          high_c: z.number().min(-50).max(60),
          low_c: z.number().min(-50).max(60)
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
