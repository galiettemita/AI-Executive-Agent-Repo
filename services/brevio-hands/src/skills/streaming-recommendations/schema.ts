import { z } from 'zod';

const ActionSchema = z.enum(['recommend', 'watchlist_add', 'watchlist_list']);

const RecommendationSchema = z.object({
  title: z.string(),
  type: z.enum(['movie', 'series']),
  genre: z.string(),
  reason: z.string(),
  available_on: z.array(z.string()).min(1)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    mood: z.string().min(2).max(80).optional(),
    genre: z.string().min(2).max(80).optional(),
    title: z.string().min(2).max(200).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'recommend' && !value.mood && !value.genre) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'STREAMING_RECOMMENDATIONS_CONTEXT_REQUIRED'
      });
    }

    if (value.action === 'watchlist_add' && !value.title) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'STREAMING_RECOMMENDATIONS_TITLE_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('streaming-recommendations'),
    action: ActionSchema,
    recommendations: z.array(RecommendationSchema).optional(),
    watchlist_added: z.boolean().optional(),
    partnership_status: z.literal('awaiting_api_partnership')
  })
  .strict();
