import { z } from 'zod';

const ActionSchema = z.enum(['route_task', 'status_report']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    task: z.string().min(5).max(2000).optional(),
    skill_hint: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'route_task' && !value.task) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'DOING_TASKS_TASK_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('doing-tasks'),
    action: ActionSchema,
    routed_skill: z.string().min(2).max(120),
    execution_plan: z.array(z.string().min(2).max(240)).min(1).max(10),
    summary: z.string().min(10).max(4096)
  })
  .strict();
