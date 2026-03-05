import { z } from 'zod';

const ActionSchema = z.enum(['log_lesson', 'weekly_review']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    lesson: z.string().min(10).max(3000).optional(),
    category: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.lesson) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SELF_IMPROVEMENT_LESSON_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('self-improvement'),
    action: ActionSchema,
    improvements: z.array(z.string().min(2).max(300)).min(1).max(10),
    next_steps: z.array(z.string().min(2).max(300)).min(1).max(10),
    summary: z.string().min(10).max(4096)
  })
  .strict();
