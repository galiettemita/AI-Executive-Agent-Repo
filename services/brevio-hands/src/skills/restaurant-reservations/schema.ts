import { z } from 'zod';

const ActionSchema = z.enum(['search', 'hold', 'book', 'reservation_status']);

const ReservationOptionSchema = z.object({
  restaurant_id: z.string(),
  name: z.string(),
  cuisine: z.string(),
  available_time: z.string(),
  estimated_total_cents: z.number().int().nonnegative()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    city: z.string().min(2).max(120).optional(),
    date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/u).optional(),
    time: z.string().regex(/^\d{2}:\d{2}$/u).optional(),
    party_size: z.number().int().min(1).max(30).optional(),
    cuisine: z.string().min(2).max(80).optional(),
    restaurant_id: z.string().min(2).max(120).optional(),
    hold_id: z.string().min(2).max(120).optional(),
    reservation_id: z.string().min(2).max(120).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search' && (!value.city || !value.date || !value.party_size)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'RESTAURANT_RESERVATIONS_SEARCH_FIELDS_REQUIRED'
      });
    }

    if (value.action === 'hold' && !value.restaurant_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'RESTAURANT_RESERVATIONS_RESTAURANT_ID_REQUIRED'
      });
    }

    if (value.action === 'book' && !value.hold_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'RESTAURANT_RESERVATIONS_HOLD_ID_REQUIRED'
      });
    }

    if (value.action === 'reservation_status' && !value.reservation_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'RESTAURANT_RESERVATIONS_RESERVATION_ID_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('restaurant-reservations'),
    action: ActionSchema,
    options: z.array(ReservationOptionSchema).optional(),
    hold_id: z.string().optional(),
    reservation_id: z.string().optional(),
    status: z.enum(['pending', 'confirmed', 'cancelled']).optional(),
    partnership_status: z.literal('awaiting_api_partnership')
  })
  .strict();
