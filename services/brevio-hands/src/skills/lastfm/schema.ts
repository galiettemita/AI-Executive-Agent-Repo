import { z } from 'zod';

const ActionSchema = z.enum(['recent_tracks', 'top_tracks', 'artist_summary']);

const TrackSchema = z.object({
  name: z.string(),
  artist: z.string(),
  playcount: z.number().int().nonnegative()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    username: z.string().min(2).max(64).optional(),
    artist: z.string().min(2).max(200).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if ((value.action === 'recent_tracks' || value.action === 'top_tracks') && !value.username) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'LASTFM_USERNAME_REQUIRED' });
    }

    if (value.action === 'artist_summary' && !value.artist) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'LASTFM_ARTIST_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('lastfm'),
    action: ActionSchema,
    tracks: z.array(TrackSchema).optional(),
    artist_summary: z
      .object({
        artist: z.string(),
        listeners: z.number().int().nonnegative(),
        summary: z.string()
      })
      .optional()
  })
  .strict();
