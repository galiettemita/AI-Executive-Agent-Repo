import { z } from 'zod';

const ActionSchema = z.enum(['track_parcel']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    tracking_number: z.string().min(5).max(80).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.tracking_number) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'POST_AT_TRACKING_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('post-at'),
    action: ActionSchema,
    tracking_number: z.string().min(5).max(80),
    latest_status: z.string().min(2).max(200),
    checkpoints: z.array(
      z
        .object({
          timestamp: z.string().datetime(),
          location: z.string().min(2).max(120),
          status: z.string().min(2).max(200)
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
