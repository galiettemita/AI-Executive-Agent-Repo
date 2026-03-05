import { z } from 'zod';

const ActionSchema = z.enum(['search_papers']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    topic: z.string().min(2).max(500).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.topic) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'LITERATURE_REVIEW_TOPIC_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('literature-review'),
    action: ActionSchema,
    papers: z.array(
      z
        .object({
          title: z.string().min(2).max(400),
          year: z.number().int().min(1900).max(2100),
          venue: z.string().min(2).max(120),
          url: z.string().url()
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
