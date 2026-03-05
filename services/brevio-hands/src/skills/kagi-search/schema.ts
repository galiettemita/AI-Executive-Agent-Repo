import { z } from 'zod';

const ActionSchema = z.enum(['search']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(500).optional(),
    max_results: z.number().int().min(1).max(20).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'KAGI_SEARCH_QUERY_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('kagi-search'),
    action: ActionSchema,
    results: z.array(
      z
        .object({
          title: z.string().min(2).max(240),
          url: z.string().url(),
          snippet: z.string().min(2).max(1000)
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
