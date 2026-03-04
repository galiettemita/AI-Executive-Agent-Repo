import { z } from 'zod';

const ActionSchema = z.enum(['monitor_topic', 'summarize_updates']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    topic: z.string().min(2).max(500).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.topic) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'PROACTIVE_RESEARCH_TOPIC_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('proactive-research'),
    action: ActionSchema,
    alerts: z.array(z.string().min(2).max(300)).min(1).max(10),
    next_check_at: z.string().datetime(),
    summary: z.string().min(10).max(4096)
  })
  .strict();
