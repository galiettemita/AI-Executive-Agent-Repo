import { z } from 'zod';

const ActionSchema = z.enum(['discover', 'play', 'pause', 'set_volume', 'group', 'status']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    speaker_id: z.string().min(2).max(120).optional(),
    query: z.string().min(2).max(240).optional(),
    volume_pct: z.number().int().min(0).max(100).optional(),
    group_with: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'play' && (!value.speaker_id || !value.query)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SONOSCLI_PLAY_FIELDS_REQUIRED' });
    }
    if (value.action === 'set_volume' && (!value.speaker_id || typeof value.volume_pct !== 'number')) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SONOSCLI_VOLUME_FIELDS_REQUIRED' });
    }
    if (value.action === 'group' && (!value.speaker_id || !value.group_with)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SONOSCLI_GROUP_FIELDS_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('sonoscli'),
    action: ActionSchema,
    zones: z.array(
      z
        .object({
          speaker_id: z.string().min(2).max(120),
          name: z.string().min(1).max(120),
          is_playing: z.boolean(),
          volume_pct: z.number().int().min(0).max(100),
          group_members: z.array(z.string().min(1).max(120)).max(10)
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
