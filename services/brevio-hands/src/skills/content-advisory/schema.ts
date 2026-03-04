import { z } from 'zod';

const ActionSchema = z.enum(['evaluate_title']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    title: z.string().min(2).max(300).optional(),
    media_type: z.enum(['movie', 'tv']).optional(),
    age_target: z.number().int().min(0).max(99).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.title) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CONTENT_ADVISORY_TITLE_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('content-advisory'),
    action: ActionSchema,
    categories: z.array(
      z
        .object({
          category: z.enum(['violence', 'language', 'substances', 'sexual_content']),
          level: z.enum(['none', 'mild', 'moderate', 'strong'])
        })
        .strict()
    ),
    overall_advisory: z.string().min(2).max(240),
    summary: z.string().min(10).max(4096)
  })
  .strict();
