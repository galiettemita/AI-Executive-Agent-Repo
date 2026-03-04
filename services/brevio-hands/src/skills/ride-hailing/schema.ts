import { z } from 'zod';

const ActionSchema = z.enum(['estimate', 'request_ride', 'ride_status', 'cancel_ride']);

const EstimateSchema = z.object({
  service_tier: z.enum(['economy', 'comfort', 'xl']),
  eta_minutes: z.number().int().positive(),
  fare_low_cents: z.number().int().nonnegative(),
  fare_high_cents: z.number().int().nonnegative()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    origin: z.string().min(3).max(200).optional(),
    destination: z.string().min(3).max(200).optional(),
    service_tier: z.enum(['economy', 'comfort', 'xl']).optional(),
    ride_id: z.string().min(2).max(120).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if ((value.action === 'estimate' || value.action === 'request_ride') && (!value.origin || !value.destination)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'RIDE_HAILING_ROUTE_REQUIRED'
      });
    }

    if ((value.action === 'ride_status' || value.action === 'cancel_ride') && !value.ride_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'RIDE_HAILING_RIDE_ID_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('ride-hailing'),
    action: ActionSchema,
    estimates: z.array(EstimateSchema).optional(),
    ride_id: z.string().optional(),
    status: z
      .enum(['requested', 'driver_assigned', 'arriving', 'in_progress', 'completed', 'cancelled'])
      .optional(),
    partnership_status: z.literal('awaiting_api_partnership')
  })
  .strict();
