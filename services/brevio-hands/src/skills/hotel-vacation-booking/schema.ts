import { z } from 'zod';

const ActionSchema = z.enum(['search_hotels', 'hold_room', 'book_room', 'reservation_status']);

const HotelOptionSchema = z.object({
  hotel_id: z.string(),
  name: z.string(),
  nightly_rate_cents: z.number().int().nonnegative(),
  total_cents: z.number().int().nonnegative(),
  refundable: z.boolean()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    city: z.string().min(2).max(120).optional(),
    check_in: z.string().regex(/^\d{4}-\d{2}-\d{2}$/u).optional(),
    check_out: z.string().regex(/^\d{4}-\d{2}-\d{2}$/u).optional(),
    guests: z.number().int().min(1).max(20).optional(),
    hotel_id: z.string().min(2).max(120).optional(),
    hold_id: z.string().min(2).max(120).optional(),
    reservation_id: z.string().min(2).max(120).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search_hotels' && (!value.city || !value.check_in || !value.check_out || !value.guests)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'HOTEL_BOOKING_SEARCH_FIELDS_REQUIRED'
      });
    }

    if (value.action === 'hold_room' && !value.hotel_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'HOTEL_BOOKING_HOTEL_ID_REQUIRED'
      });
    }

    if (value.action === 'book_room' && !value.hold_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'HOTEL_BOOKING_HOLD_ID_REQUIRED'
      });
    }

    if (value.action === 'reservation_status' && !value.reservation_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'HOTEL_BOOKING_RESERVATION_ID_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('hotel-vacation-booking'),
    action: ActionSchema,
    hotels: z.array(HotelOptionSchema).optional(),
    hold_id: z.string().optional(),
    reservation_id: z.string().optional(),
    status: z.enum(['pending', 'confirmed', 'cancelled']).optional(),
    partnership_status: z.literal('awaiting_api_partnership')
  })
  .strict();
