import { z } from 'zod';

const ActionSchema = z.enum(['build_plan', 'log_session']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    goal: z.string().min(2).max(240).optional(),
    weeks: z.number().int().min(1).max(52).optional(),
    session_notes: z.string().min(5).max(4000).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'build_plan' && !value.goal) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CLAWD_COACH_GOAL_REQUIRED' });
    }
    if (value.action === 'log_session' && !value.session_notes) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CLAWD_COACH_SESSION_NOTES_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('clawd-coach'),
    action: ActionSchema,
    workouts: z.array(z.string().min(2).max(200)).min(1).max(20),
    milestones: z.array(z.string().min(2).max(200)).min(1).max(20),
    summary: z.string().min(10).max(4096)
  })
  .strict();
