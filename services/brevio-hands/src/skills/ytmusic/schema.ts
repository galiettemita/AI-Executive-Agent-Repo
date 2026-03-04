import { z } from 'zod';

const ActionSchema = z.enum(['search', 'play', 'queue']);

const TrackSchema = z.object({
  id: z.string(),
  title: z.string(),
  artist: z.string()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(300).optional(),
    track_id: z.string().min(2).max(100).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search' && !value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'YTMUSIC_QUERY_REQUIRED' });
    }

    if ((value.action === 'play' || value.action === 'queue') && !value.track_id && !value.query) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'YTMUSIC_TRACK_TARGET_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('ytmusic'),
    action: ActionSchema,
    tracks: z.array(TrackSchema).optional(),
    now_playing: TrackSchema.optional(),
    queued: z.boolean().optional()
  })
  .strict();
