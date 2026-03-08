import { z } from 'zod';

const ActionSchema = z.enum(['search_providers', 'request_quote', 'book_service', 'booking_status']);

const ProviderSchema = z.object({
  provider_id: z.string(),
  name: z.string(),
  service_type: z.string(),
  rating: z.number().min(0).max(5),
  estimated_start_cents: z.number().int().nonnegative()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    service_type: z.string().min(2).max(120).optional(),
    zip_code: z.string().regex(/^\d{5}$/u).optional(),
    provider_id: z.string().min(2).max(120).optional(),
    booking_id: z.string().min(2).max(120).optional(),
    preferred_time: z.string().min(2).max(80).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search_providers' && (!value.service_type || !value.zip_code)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'LOCAL_SERVICE_SEARCH_FIELDS_REQUIRED'
      });
    }

    if (value.action === 'request_quote' && !value.provider_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'LOCAL_SERVICE_PROVIDER_ID_REQUIRED'
      });
    }

    if (value.action === 'book_service' && !value.provider_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'LOCAL_SERVICE_PROVIDER_ID_REQUIRED'
      });
    }

    if (value.action === 'booking_status' && !value.booking_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'LOCAL_SERVICE_BOOKING_ID_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('local-service-booking'),
    action: ActionSchema,
    providers: z.array(ProviderSchema).optional(),
    booking_id: z.string().optional(),
    status: z.enum(['quote_pending', 'scheduled', 'completed', 'cancelled']).optional(),
    partnership_status: z.literal('awaiting_api_partnership')
  })
  .strict();
