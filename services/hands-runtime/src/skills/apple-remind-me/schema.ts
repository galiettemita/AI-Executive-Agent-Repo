import { z } from 'zod';

const ActionSchema = z.enum(['create', 'list', 'complete', 'delete']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    title: z.string().min(2).max(240).optional(),
    due_at: z.string().datetime().optional(),
    reminder_id: z.string().min(3).max(120).optional(),
    list: z.string().min(1).max(80).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'create' && !value.title) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'APPLE_REMIND_ME_TITLE_REQUIRED' });
    }
    if ((value.action === 'complete' || value.action === 'delete') && !value.reminder_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'APPLE_REMIND_ME_ID_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('apple-reminders'),
    action: ActionSchema,
    reminders: z.array(
      z
        .object({
          reminder_id: z.string().min(3).max(120),
          title: z.string().min(1).max(240),
          due_at: z.string().datetime().optional(),
          list: z.string().min(1).max(80),
          status: z.enum(['open', 'completed'])
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
