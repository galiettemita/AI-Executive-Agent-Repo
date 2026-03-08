import { z } from 'zod';

const ActionSchema = z.enum(['create_event', 'list_events', 'update_event', 'cancel_event']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    event_id: z.string().min(3).max(120).optional(),
    title: z.string().min(2).max(240).optional(),
    start_at: z.string().datetime().optional(),
    end_at: z.string().datetime().optional(),
    calendar: z.string().min(1).max(80).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'create_event' && (!value.title || !value.start_at || !value.end_at)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CALCTL_EVENT_FIELDS_REQUIRED' });
    }
    if (value.action === 'update_event' && (!value.event_id || !value.title)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CALCTL_UPDATE_FIELDS_REQUIRED' });
    }
    if (value.action === 'cancel_event' && !value.event_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CALCTL_EVENT_ID_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('apple-calendar'),
    action: ActionSchema,
    events: z.array(
      z
        .object({
          event_id: z.string().min(3).max(120),
          title: z.string().min(1).max(240),
          start_at: z.string().datetime(),
          end_at: z.string().datetime(),
          calendar: z.string().min(1).max(80),
          status: z.enum(['confirmed', 'cancelled'])
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
