import { z } from 'zod';

const ActionSchema = z.enum(['family_schedule', 'pickup_plan', 'location_checkin']);

const EventSchema = z.object({
  event_id: z.string(),
  title: z.string(),
  date: z.string(),
  time: z.string(),
  location: z.string()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    child_name: z.string().min(2).max(80).optional(),
    date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/u).optional(),
    location: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'pickup_plan' && (!value.child_name || !value.date)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'KIDS_FAMILY_PICKUP_FIELDS_REQUIRED'
      });
    }

    if (value.action === 'location_checkin' && (!value.child_name || !value.location)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'KIDS_FAMILY_CHECKIN_FIELDS_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('kids-family-management'),
    action: ActionSchema,
    events: z.array(EventSchema).optional(),
    checkin_status: z.enum(['on_time', 'delayed', 'arrived']).optional(),
    partnership_status: z.literal('awaiting_api_partnership')
  })
  .strict();
