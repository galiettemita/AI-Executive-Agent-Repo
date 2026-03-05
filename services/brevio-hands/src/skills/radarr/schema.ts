import { z } from 'zod';

const ActionSchema = z.enum(['search_movie', 'add_movie', 'list_queue']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(240).optional(),
    tmdb_id: z.string().min(2).max(120).optional(),
    quality_profile: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search_movie' && !value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'RADARR_QUERY_REQUIRED' });
    }
    if (value.action === 'add_movie' && !value.tmdb_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'RADARR_TMDB_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('radarr'),
    action: ActionSchema,
    movies: z.array(
      z
        .object({
          movie_id: z.string().min(2).max(120),
          title: z.string().min(1).max(240),
          status: z.enum(['queued', 'monitored']),
          quality_profile: z.string().min(2).max(120)
        })
        .strict()
    ),
    queue_count: z.number().int().min(0).max(10000),
    summary: z.string().min(10).max(4096)
  })
  .strict();
