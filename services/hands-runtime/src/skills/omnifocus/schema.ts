import { z } from 'zod';

const ActionSchema = z.enum(['add_task', 'list_flagged', 'complete_task', 'defer_task']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    title: z.string().min(2).max(240).optional(),
    task_id: z.string().min(3).max(120).optional(),
    project: z.string().min(1).max(120).optional(),
    defer_until: z.string().datetime().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'add_task' && !value.title) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'OMNIFOCUS_TITLE_REQUIRED' });
    }
    if ((value.action === 'complete_task' || value.action === 'defer_task') && !value.task_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'OMNIFOCUS_TASK_REQUIRED' });
    }
    if (value.action === 'defer_task' && !value.defer_until) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'OMNIFOCUS_DEFER_UNTIL_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('omnifocus'),
    action: ActionSchema,
    tasks: z.array(
      z
        .object({
          task_id: z.string().min(3).max(120),
          title: z.string().min(1).max(240),
          project: z.string().min(1).max(120),
          status: z.enum(['available', 'completed', 'deferred']),
          defer_until: z.string().datetime().optional()
        })
        .strict()
    ),
    flagged_count: z.number().int().min(0).max(10000),
    summary: z.string().min(10).max(4096)
  })
  .strict();
