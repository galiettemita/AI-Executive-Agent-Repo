import { z } from 'zod';

const ActionSchema = z.enum(['get_measurements', 'trend_summary']);
const MeasureTypeSchema = z.enum(['weight', 'body_fat_pct', 'muscle_mass_kg', 'heart_rate_bpm']);

const MeasurementSchema = z.object({
  recorded_at: z.string().datetime(),
  measure_type: MeasureTypeSchema,
  value: z.number(),
  unit: z.string().min(1).max(20)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    measure_type: MeasureTypeSchema.optional(),
    start_date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).optional(),
    end_date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.measure_type) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'WITHINGS_MEASURE_TYPE_REQUIRED' });
    }

    if ((value.start_date && !value.end_date) || (!value.start_date && value.end_date)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'WITHINGS_DATE_RANGE_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('withings-health'),
    action: ActionSchema,
    measure_type: MeasureTypeSchema,
    measurements: z.array(MeasurementSchema).max(365),
    trend: z.enum(['up', 'down', 'stable']),
    summary: z.string().min(10).max(4096)
  })
  .strict();
