import { z } from 'zod';

const ActionSchema = z.enum(['launch_app', 'key_press', 'status']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    device_id: z.string().min(2).max(120).optional(),
    app_id: z.string().min(1).max(120).optional(),
    key: z.string().min(1).max(40).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'launch_app' && (!value.device_id || !value.app_id)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'ROKU_ACTION_FIELDS_REQUIRED' });
    }
    if (value.action === 'key_press' && (!value.device_id || !value.key)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'ROKU_ACTION_FIELDS_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('roku'),
    action: ActionSchema,
    device_id: z.string().min(2).max(120),
    current_app: z.string().min(1).max(120),
    power_state: z.enum(['on', 'off']),
    summary: z.string().min(10).max(4096)
  })
  .strict();
