import { z } from 'zod';

const FlightRecordSchema = z.object({
  flight: z.string(),
  airline: z.string(),
  status: z.enum(['scheduled', 'active', 'landed', 'delayed']),
  departure_airport: z.string(),
  arrival_airport: z.string(),
  gate: z.string().optional(),
  terminal: z.string().optional(),
  delay_minutes: z.number().int().nonnegative().optional()
});

export const InputSchema = z
  .object({
    flight_iata: z.string().regex(/^[A-Z0-9]{2,8}$/u).optional(),
    flight_icao: z.string().regex(/^[A-Z0-9]{3,8}$/u).optional(),
    airline_iata: z.string().regex(/^[A-Z0-9]{2,3}$/u).optional(),
    date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/u).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.flight_iata && !value.flight_icao && !value.airline_iata) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'AVIATIONSTACK_FLIGHT_IDENTIFIER_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('aviationstack'),
    flights: z.array(FlightRecordSchema),
    queried_at_utc: z.string().datetime()
  })
  .strict();
