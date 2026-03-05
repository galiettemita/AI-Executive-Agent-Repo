import { z } from 'zod';

const ActionSchema = z.enum(['providers', 'book_visit', 'booking_status']);

const ProviderSchema = z.object({
  provider_id: z.string(),
  name: z.string(),
  service_type: z.string(),
  rating: z.number().min(0).max(5)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    pet_type: z.string().min(2).max(80).optional(),
    service_type: z.string().min(2).max(80).optional(),
    provider_id: z.string().min(2).max(120).optional(),
    booking_id: z.string().min(2).max(120).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'providers' && (!value.pet_type || !value.service_type)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'PET_CARE_PROVIDER_FIELDS_REQUIRED'
      });
    }

    if (value.action === 'book_visit' && !value.provider_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'PET_CARE_PROVIDER_ID_REQUIRED'
      });
    }

    if (value.action === 'booking_status' && !value.booking_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'PET_CARE_BOOKING_ID_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('pet-care'),
    action: ActionSchema,
    providers: z.array(ProviderSchema).optional(),
    booking_id: z.string().optional(),
    status: z.enum(['scheduled', 'in_progress', 'completed', 'cancelled']).optional(),
    partnership_status: z.literal('awaiting_api_partnership')
  })
  .strict();
