import { z } from 'zod';

const ActionSchema = z.enum(['power_on', 'power_off', 'launch_app', 'set_volume', 'status']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    device_id: z.string().min(3).max(120).optional(),
    app_id: z.string().min(2).max(120).optional(),
    volume_pct: z.number().int().min(0).max(100).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'launch_app' && !value.app_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SAMSUNG_SMART_TV_APP_REQUIRED' });
    }
    if (value.action === 'set_volume' && typeof value.volume_pct !== 'number') {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SAMSUNG_SMART_TV_VOLUME_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('samsung-smart-tv'),
    action: ActionSchema,
    device_id: z.string().min(3).max(120),
    power_state: z.enum(['on', 'off']),
    current_app: z.string().min(1).max(120),
    volume_pct: z.number().int().min(0).max(100),
    summary: z.string().min(10).max(4096)
  })
  .strict();
