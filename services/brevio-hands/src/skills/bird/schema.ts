import { z } from 'zod';

const ActionSchema = z.enum(['timeline', 'search', 'post']);

const PostSchema = z.object({
  id: z.string(),
  author: z.string(),
  text: z.string(),
  likes: z.number().int().nonnegative(),
  reposts: z.number().int().nonnegative()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(280).optional(),
    text: z.string().min(1).max(280).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search' && !value.query) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'BIRD_QUERY_REQUIRED'
      });
    }

    if (value.action === 'post' && !value.text) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'BIRD_POST_TEXT_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('bird'),
    action: ActionSchema,
    posts: z.array(PostSchema).optional(),
    posted: z.boolean().optional(),
    post_id: z.string().optional()
  })
  .strict();
