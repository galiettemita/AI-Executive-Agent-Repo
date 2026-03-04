import { z } from 'zod';

const iataCode = z.string().regex(/^[A-Z]{3}$/u, 'must be a 3-letter IATA code');

const FlightRecordSchema = z.object({
  callsign: z.string(),
  icao24: z.string(),
  origin: iataCode,
  destination: iataCode,
  altitude_m: z.number().int().nonnegative(),
  speed_kts: z.number().int().nonnegative(),
  status: z.enum(['airborne', 'scheduled', 'landed'])
});

export const InputSchema = z
  .object({
    callsign: z.string().regex(/^[A-Z0-9]{3,8}$/u).optional(),
    icao24: z.string().regex(/^[a-f0-9]{6}$/u).optional(),
    origin_iata: iataCode.optional(),
    destination_iata: iataCode.optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.callsign && !value.icao24 && !value.origin_iata && !value.destination_iata) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'FLIGHT_TRACKER_IDENTIFIER_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('opensky'),
    flights: z.array(FlightRecordSchema),
    queried_at_utc: z.string().datetime()
  })
  .strict();
