import { z } from 'zod';

const ActionSchema = z.enum(['history', 'trending', 'mark_watched']);
const MediaTypeSchema = z.enum(['movie', 'show']);

const ItemSchema = z.object({
  id: z.string(),
  title: z.string(),
  media_type: MediaTypeSchema,
  year: z.number().int().optional()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    media_id: z.string().min(2).max(120).optional(),
    media_type: MediaTypeSchema.optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'mark_watched' && !value.media_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'TRAKT_MEDIA_ID_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('trakt'),
    action: ActionSchema,
    items: z.array(ItemSchema).optional(),
    marked_watched: z.boolean().optional()
  })
  .strict();
