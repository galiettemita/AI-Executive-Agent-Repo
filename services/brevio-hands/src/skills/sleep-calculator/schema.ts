import { z } from 'zod';

const ActionSchema = z.enum(['bedtime_from_wake', 'wake_from_bedtime']);

const RecommendationSchema = z.object({
  target_time_local: z.string().regex(/^\d{2}:\d{2}$/),
  sleep_cycles: z.number().int().min(1).max(8),
  hours_in_bed: z.number().min(1).max(12)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    wake_time_local: z.string().regex(/^\d{2}:\d{2}$/).optional(),
    bedtime_local: z.string().regex(/^\d{2}:\d{2}$/).optional(),
    sleep_cycle_minutes: z.number().int().min(60).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'bedtime_from_wake' && !value.wake_time_local) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SLEEP_CALCULATOR_WAKE_TIME_REQUIRED' });
    }

    if (value.action === 'wake_from_bedtime' && !value.bedtime_local) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SLEEP_CALCULATOR_BEDTIME_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('sleep-calculator'),
    action: ActionSchema,
    recommendations: z.array(RecommendationSchema).max(8),
    summary: z.string().min(10).max(4096)
  })
  .strict();
