import { z } from 'zod';

const ActionSchema = z.enum(['coach_message', 'conflict_plan']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    context: z.string().min(10).max(4000).optional(),
    goal: z.string().min(5).max(500).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.context || !value.goal) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'RELATIONSHIP_SKILLS_CONTEXT_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('relationship-skills'),
    action: ActionSchema,
    talking_points: z.array(z.string().min(2).max(300)).min(2).max(10),
    suggested_message: z.string().min(10).max(2000),
    summary: z.string().min(10).max(4096)
  })
  .strict();
