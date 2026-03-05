import { z } from 'zod';

const ActionSchema = z.enum(['search', 'play', 'add_to_playlist']);

const TrackSchema = z.object({
  id: z.string(),
  title: z.string(),
  artist: z.string(),
  album: z.string()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(300).optional(),
    playlist_id: z.string().min(2).max(100).optional(),
    track_id: z.string().min(2).max(100).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search' && !value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'APPLE_MUSIC_QUERY_REQUIRED' });
    }

    if (value.action === 'play' && !value.track_id && !value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'APPLE_MUSIC_PLAY_TARGET_REQUIRED' });
    }

    if (value.action === 'add_to_playlist' && (!value.track_id || !value.playlist_id)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'APPLE_MUSIC_PLAYLIST_FIELDS_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('apple-music'),
    action: ActionSchema,
    tracks: z.array(TrackSchema).optional(),
    now_playing: TrackSchema.optional(),
    playlist_updated: z.boolean().optional()
  })
  .strict();
