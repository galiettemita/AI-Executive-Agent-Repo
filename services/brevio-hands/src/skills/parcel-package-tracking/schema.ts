import { z } from 'zod';

const HistorySchema = z.object({
  timestamp: z.string().datetime(),
  location: z.string(),
  description: z.string()
});

export const InputSchema = z
  .object({
    tracking_number: z.string().min(8).max(40),
    carrier: z.enum(['auto', 'ups', 'usps', 'fedex', 'dhl']).optional(),
    locale: z.string().min(2).max(10).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.tracking_number.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'PARCEL_TRACKING_NUMBER_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('parcel'),
    tracking_number: z.string(),
    carrier: z.enum(['ups', 'usps', 'fedex', 'dhl']),
    status: z.enum(['label_created', 'in_transit', 'out_for_delivery', 'delivered']),
    eta: z.string().datetime().optional(),
    history: z.array(HistorySchema)
  })
  .strict();
