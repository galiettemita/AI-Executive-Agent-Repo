import { z } from 'zod';

const ActionSchema = z.enum(['discover_devices', 'cast_media', 'pause', 'resume', 'stop', 'status']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    device_name: z.string().min(2).max(120).optional(),
    media_url: z.string().url().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'cast_media' && (!value.device_name || !value.media_url)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CHROMECAST_CAST_FIELDS_REQUIRED' });
    }
    if ((value.action === 'pause' || value.action === 'resume' || value.action === 'stop') && !value.device_name) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CHROMECAST_DEVICE_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('chromecast'),
    action: ActionSchema,
    devices: z.array(
      z
        .object({
          device_name: z.string().min(2).max(120),
          is_active: z.boolean(),
          last_media_url: z.string().url().optional()
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
