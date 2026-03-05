import { z } from 'zod';

const ActionSchema = z.enum(['add_task', 'list_tasks', 'complete_task', 'delete_task']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    task_content: z.string().min(2).max(500).optional(),
    task_id: z.string().min(3).max(120).optional(),
    project_id: z.string().min(1).max(120).optional(),
    due_date: z.string().datetime().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'add_task' && !value.task_content) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'TICKTICK_TASK_CONTENT_REQUIRED' });
    }
    if ((value.action === 'complete_task' || value.action === 'delete_task') && !value.task_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'TICKTICK_TASK_ID_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('ticktick'),
    action: ActionSchema,
    tasks: z.array(
      z
        .object({
          task_id: z.string().min(3).max(120),
          content: z.string().min(1).max(500),
          project_id: z.string().min(1).max(120),
          status: z.enum(['open', 'completed']),
          due_date: z.string().datetime().optional()
        })
        .strict()
    ),
    total_tasks: z.number().int().min(0).max(5000),
    summary: z.string().min(10).max(4096)
  })
  .strict();
