import { z } from 'zod';

const ActionSchema = z.enum(['play', 'pause', 'next', 'previous', 'set_volume', 'status']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(200).optional(),
    volume_pct: z.number().int().min(0).max(100).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'play' && !value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SPOTIFY_QUERY_REQUIRED' });
    }
    if (value.action === 'set_volume' && typeof value.volume_pct !== 'number') {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SPOTIFY_VOLUME_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('spotify'),
    action: ActionSchema,
    now_playing: z.object({
      track: z.string().min(1).max(200),
      artist: z.string().min(1).max(200),
      is_playing: z.boolean(),
      volume_pct: z.number().int().min(0).max(100),
      device: z.string().min(1).max(120)
    }),
    summary: z.string().min(10).max(4096)
  })
  .strict();
