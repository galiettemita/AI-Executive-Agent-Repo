import { z } from 'zod';

const ActionSchema = z.enum(['search', 'play', 'recent']);

const MediaSchema = z.object({
  id: z.string(),
  title: z.string(),
  type: z.enum(['movie', 'episode', 'album']),
  year: z.number().int().optional()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(300).optional(),
    media_id: z.string().min(2).max(100).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search' && !value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'PLEX_QUERY_REQUIRED' });
    }

    if (value.action === 'play' && !value.media_id && !value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'PLEX_PLAY_TARGET_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('plex'),
    action: ActionSchema,
    results: z.array(MediaSchema).optional(),
    now_playing: MediaSchema.optional()
  })
  .strict();
