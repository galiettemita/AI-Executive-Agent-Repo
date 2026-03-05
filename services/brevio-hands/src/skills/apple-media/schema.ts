import { z } from 'zod';

const ActionSchema = z.enum(['discover_devices', 'playback_status', 'control_playback']);
const CommandSchema = z.enum(['play', 'pause', 'next', 'previous', 'set_volume']);

const DeviceSchema = z.object({
  device_name: z.string().min(2).max(120),
  device_type: z.enum(['apple_tv', 'homepod', 'airplay_speaker']),
  is_active: z.boolean()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    device_name: z.string().min(2).max(120).optional(),
    command: CommandSchema.optional(),
    volume_pct: z.number().int().min(0).max(100).optional(),
    source: z.string().min(2).max(160).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if ((value.action === 'playback_status' || value.action === 'control_playback') && !value.device_name) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'APPLE_MEDIA_DEVICE_REQUIRED' });
    }

    if (value.action === 'control_playback' && !value.command) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'APPLE_MEDIA_COMMAND_REQUIRED' });
    }

    if (value.command === 'set_volume' && typeof value.volume_pct !== 'number') {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'APPLE_MEDIA_VOLUME_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('apple-media'),
    action: ActionSchema,
    devices: z.array(DeviceSchema).max(20),
    now_playing: z
      .object({
        title: z.string().min(1).max(200),
        artist: z.string().min(1).max(200),
        position_seconds: z.number().int().min(0),
        is_playing: z.boolean(),
        volume_pct: z.number().int().min(0).max(100)
      })
      .optional(),
    applied_command: CommandSchema.optional(),
    summary: z.string().min(10).max(4096)
  })
  .strict();
