import { z } from 'zod';

export const InputSchema = z
  .object({
    action: z.enum(['playback', 'search', 'history', 'top_tracks']),
    query: z.string().min(1).max(200).optional()
  })
  .strict();

const TrackSchema = z.object({
  track_id: z.string(),
  name: z.string(),
  artist: z.string()
});

export const OutputSchema = z
  .object({
    provider: z.literal('spotify-web-api'),
    action: z.enum(['playback', 'search', 'history', 'top_tracks']),
    playing: z
      .object({
        track_id: z.string(),
        name: z.string(),
        artist: z.string(),
        progress_ms: z.number().int().nonnegative()
      })
      .optional(),
    results: z.array(TrackSchema).optional()
  })
  .strict();
