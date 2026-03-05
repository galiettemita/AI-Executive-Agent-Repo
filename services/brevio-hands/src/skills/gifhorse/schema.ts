import { z } from 'zod';

const ActionSchema = z.enum(['search_gif']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(200).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'GIFHORSE_QUERY_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('gifhorse'),
    action: ActionSchema,
    gifs: z.array(
      z
        .object({
          caption: z.string().min(2).max(200),
          gif_url: z.string().url()
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
