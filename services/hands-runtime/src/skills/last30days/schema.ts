import { z } from 'zod';

const ActionSchema = z.enum(['scan_topic']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(500).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'LAST30DAYS_QUERY_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('last30days'),
    action: ActionSchema,
    highlights: z.array(z.string().min(2).max(300)).min(1).max(10),
    sources: z.array(z.string().url()).min(1).max(10),
    summary: z.string().min(10).max(4096)
  })
  .strict();
