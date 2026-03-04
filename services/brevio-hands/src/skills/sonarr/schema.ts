import { z } from 'zod';

const ActionSchema = z.enum(['search_series', 'add_series', 'list_queue']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(240).optional(),
    tvdb_id: z.string().min(2).max(120).optional(),
    quality_profile: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search_series' && !value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SONARR_QUERY_REQUIRED' });
    }
    if (value.action === 'add_series' && !value.tvdb_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SONARR_TVDB_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('sonarr'),
    action: ActionSchema,
    series: z.array(
      z
        .object({
          series_id: z.string().min(2).max(120),
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
