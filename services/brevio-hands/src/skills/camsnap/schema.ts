import { z } from 'zod';

const ActionSchema = z.enum(['capture_frame', 'capture_clip']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    camera_id: z.string().min(2).max(120).optional(),
    duration_seconds: z.number().int().min(1).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.camera_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CAMSNAP_CAMERA_REQUIRED' });
    }
    if (value.action === 'capture_clip' && typeof value.duration_seconds !== 'number') {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CAMSNAP_DURATION_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('camsnap'),
    action: ActionSchema,
    media_url: z.string().url(),
    captured_at_utc: z.string().datetime(),
    resolution: z.string().min(2).max(40),
    summary: z.string().min(10).max(4096)
  })
  .strict();
