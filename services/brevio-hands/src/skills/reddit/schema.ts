import { z } from 'zod';

const ActionSchema = z.enum(['search', 'list_hot', 'post']);

const PostSummarySchema = z.object({
  id: z.string(),
  subreddit: z.string(),
  title: z.string(),
  score: z.number().int(),
  url: z.string().url()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    subreddit: z.string().min(2).max(64).optional(),
    query: z.string().min(2).max(300).optional(),
    title: z.string().min(1).max(300).optional(),
    text: z.string().min(1).max(40000).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search' && !value.query) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'REDDIT_QUERY_REQUIRED'
      });
    }

    if (value.action === 'post' && (!value.subreddit || !value.title || !value.text)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'REDDIT_POST_FIELDS_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('reddit'),
    action: ActionSchema,
    posts: z.array(PostSummarySchema).optional(),
    submitted: z.boolean().optional(),
    post_id: z.string().optional()
  })
  .strict();
