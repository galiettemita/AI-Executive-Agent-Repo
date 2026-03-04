import { z } from 'zod';

const ActionSchema = z.enum(['top_tracks', 'top_artists', 'listening_summary']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    window: z.enum(['4w', '6m', '12m']).optional(),
    limit: z.number().int().min(1).max(50).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('spotify-history'),
    action: ActionSchema,
    top_tracks: z
      .array(
        z.object({
          title: z.string().min(1).max(200),
          artist: z.string().min(1).max(200),
          play_count: z.number().int().nonnegative()
        })
      )
      .max(100),
    top_artists: z
      .array(
        z.object({
          name: z.string().min(1).max(200),
          play_count: z.number().int().nonnegative()
        })
      )
      .max(100),
    total_listening_minutes: z.number().int().nonnegative(),
    summary: z.string().min(10).max(4096)
  })
  .strict();
