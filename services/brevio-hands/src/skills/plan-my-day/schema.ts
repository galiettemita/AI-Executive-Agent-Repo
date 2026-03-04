import { z } from 'zod';

const ActionSchema = z.enum(['build_plan', 'rebalance_plan']);

const TaskSchema = z.object({
  title: z.string().min(2).max(180),
  duration_minutes: z.number().int().min(10).max(240),
  priority: z.enum(['high', 'medium', 'low']),
  energy: z.enum(['deep', 'admin'])
});

const WindowSchema = z.object({
  start_local: z.string().regex(/^\d{2}:\d{2}$/),
  end_local: z.string().regex(/^\d{2}:\d{2}$/)
});

const TimeBlockSchema = z.object({
  title: z.string().min(2).max(180),
  start_local: z.string().regex(/^\d{2}:\d{2}$/),
  end_local: z.string().regex(/^\d{2}:\d{2}$/),
  source: z.enum(['task', 'break', 'buffer'])
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    timezone: z.string().min(3).max(80),
    date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/),
    tasks: z.array(TaskSchema).max(50).optional(),
    available_windows: z.array(WindowSchema).max(10).optional(),
    disruptions: z.array(z.string().min(2).max(180)).max(10).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'build_plan' && !value.tasks?.length) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'PLAN_MY_DAY_TASKS_REQUIRED'
      });
    }

    if (value.action === 'rebalance_plan' && !value.disruptions?.length) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'PLAN_MY_DAY_DISRUPTIONS_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('plan-my-day'),
    action: ActionSchema,
    time_blocks: z.array(TimeBlockSchema).max(30),
    overflow_tasks: z.array(z.string().min(2).max(180)).max(20),
    strategy_notes: z.array(z.string().min(2).max(240)).max(10)
  })
  .strict();
