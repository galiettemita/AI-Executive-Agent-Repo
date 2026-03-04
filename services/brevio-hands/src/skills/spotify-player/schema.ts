import { z } from 'zod';

const ActionSchema = z.enum(['search_tracks', 'queue_track', 'playback_status']);

const TrackSchema = z.object({
  track_id: z.string().min(2).max(120),
  title: z.string().min(1).max(200),
  artist: z.string().min(1).max(200),
  duration_seconds: z.number().int().positive()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(200).optional(),
    track_id: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search_tracks' && !value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SPOTIFY_PLAYER_QUERY_REQUIRED' });
    }
    if (value.action === 'queue_track' && !value.track_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SPOTIFY_PLAYER_TRACK_ID_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('spotify-player'),
    action: ActionSchema,
    tracks: z.array(TrackSchema).max(100),
    queue_length: z.number().int().min(0),
    summary: z.string().min(10).max(4096)
  })
  .strict();
