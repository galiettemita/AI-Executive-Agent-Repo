import { z } from 'zod';

const ActionSchema = z.enum([
  'detect_reorder_need',
  'proactive_recommendations',
  'schedule_reorder_reminder'
]);

const HouseholdItemSchema = z.object({
  name: z.string().min(1).max(120),
  days_since_last_order: z.number().int().min(0).max(3650),
  typical_cycle_days: z.number().int().min(1).max(3650),
  estimated_units_left: z.number().int().min(0).max(10000)
});

const RecommendationSchema = z.object({
  item: z.string().min(1).max(120),
  urgency: z.enum(['low', 'medium', 'high']),
  reason: z.string().min(5).max(240)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    household_items: z.array(HouseholdItemSchema).max(200).optional(),
    reminder_time_local: z.string().regex(/^\d{2}:\d{2}$/).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if ((value.action === 'detect_reorder_need' || value.action === 'proactive_recommendations') && !value.household_items?.length) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CLAWRINGHOUSE_ITEMS_REQUIRED' });
    }

    if (value.action === 'schedule_reorder_reminder' && !value.reminder_time_local) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CLAWRINGHOUSE_REMINDER_TIME_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('clawringhouse'),
    action: ActionSchema,
    recommendations: z.array(RecommendationSchema).max(200),
    next_reminder_local: z.string().regex(/^\d{2}:\d{2}$/).optional(),
    summary: z.string().min(10).max(4096)
  })
  .strict();
