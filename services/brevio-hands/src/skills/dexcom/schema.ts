import { z } from 'zod';

const ActionSchema = z.enum(['glucose_readings', 'trend_alerts']);

const ReadingSchema = z.object({
  timestamp: z.string().datetime(),
  value_mg_dl: z.number().int().min(20).max(500),
  trend: z.enum(['rising', 'falling', 'steady']),
  state: z.enum(['low', 'in_range', 'high'])
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    start_time: z.string().datetime().optional(),
    end_time: z.string().datetime().optional(),
    minutes: z.number().int().min(5).max(1440).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    const hasRange = Boolean(value.start_time && value.end_time);
    const hasWindow = Boolean(value.minutes);
    if (!hasRange && !hasWindow) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'DEXCOM_TIME_RANGE_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('dexcom'),
    action: ActionSchema,
    readings: z.array(ReadingSchema).max(500),
    alerts: z.array(z.string().min(2).max(240)).max(20),
    summary: z.string().min(10).max(4096)
  })
  .strict();
