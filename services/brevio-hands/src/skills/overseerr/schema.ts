import { z } from 'zod';

const ActionSchema = z.enum(['search_media', 'request_media', 'list_requests']);
const MediaTypeSchema = z.enum(['movie', 'tv']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(240).optional(),
    media_type: MediaTypeSchema.optional(),
    media_id: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search_media' && !value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'OVERSEERR_QUERY_REQUIRED' });
    }
    if (value.action === 'request_media' && !value.media_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'OVERSEERR_MEDIA_ID_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('overseerr'),
    action: ActionSchema,
    requests: z.array(
      z
        .object({
          request_id: z.string().min(2).max(120),
          media_id: z.string().min(2).max(120),
          title: z.string().min(1).max(240),
          media_type: MediaTypeSchema,
          status: z.enum(['pending', 'approved'])
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
