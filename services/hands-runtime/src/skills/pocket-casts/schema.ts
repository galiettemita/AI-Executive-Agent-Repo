import { z } from 'zod';

const ActionSchema = z.enum(['queue_from_youtube', 'list_queue', 'remove_episode']);

const EpisodeSchema = z.object({
  id: z.string(),
  title: z.string(),
  source: z.string()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    youtube_url: z.string().url().optional(),
    episode_id: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'queue_from_youtube' && !value.youtube_url) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'POCKET_CASTS_YOUTUBE_URL_REQUIRED'
      });
    }

    if (value.action === 'remove_episode' && !value.episode_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'POCKET_CASTS_EPISODE_ID_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('pocket-casts'),
    action: ActionSchema,
    queue: z.array(EpisodeSchema).optional(),
    queued: z.boolean().optional(),
    removed: z.boolean().optional()
  })
  .strict();
