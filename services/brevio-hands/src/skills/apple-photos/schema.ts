import { z } from 'zod';

const ActionSchema = z.enum(['list_albums', 'search_photos', 'recent_photos']);

const PhotoSchema = z.object({
  photo_id: z.string().min(2).max(80),
  filename: z.string().min(2).max(200),
  captured_at: z.string().datetime(),
  album: z.string().min(2).max(120),
  tags: z.array(z.string().min(1).max(40)).max(20)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    album_name: z.string().min(2).max(120).optional(),
    query: z.string().min(2).max(120).optional(),
    date_from: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).optional(),
    date_to: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).optional(),
    limit: z.number().int().min(1).max(100).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search_photos' && !value.query?.trim()) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'APPLE_PHOTOS_QUERY_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('apple-photos'),
    action: ActionSchema,
    albums: z.array(z.string().min(2).max(120)).max(200),
    photos: z.array(PhotoSchema).max(500),
    summary: z.string().min(10).max(4096)
  })
  .strict();
