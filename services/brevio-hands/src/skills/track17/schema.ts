import { z } from 'zod';

const CheckpointSchema = z.object({
  timestamp: z.string().datetime(),
  location: z.string(),
  status: z.string()
});

export const InputSchema = z
  .object({
    tracking_number: z.string().min(8).max(40),
    carrier_code: z.string().min(2).max(10).optional(),
    request_locale: z.string().min(2).max(10).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.tracking_number.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'TRACK17_TRACKING_NUMBER_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('17track'),
    tracking_number: z.string(),
    carrier: z.string(),
    status: z.enum(['not_found', 'in_transit', 'out_for_delivery', 'delivered']),
    checkpoints: z.array(CheckpointSchema)
  })
  .strict();
