import { z } from 'zod';

const ActionSchema = z.enum(['search_flights', 'compare_itineraries']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    origin: z.string().min(3).max(3).optional(),
    destination: z.string().min(3).max(3).optional(),
    depart_date: z.string().date().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.origin || !value.destination || !value.depart_date) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'AEROBASE_ROUTE_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('aerobase-skill'),
    action: ActionSchema,
    itineraries: z.array(
      z
        .object({
          flight_number: z.string().min(2).max(20),
          duration_minutes: z.number().int().min(30).max(2000),
          price_usd: z.number().min(1).max(50000),
          jetlag_score: z.number().int().min(0).max(100)
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
