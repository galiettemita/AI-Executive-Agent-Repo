import { z } from 'zod';

const ActionSchema = z.enum(['sync_steps', 'sync_sleep', 'sync_heart_rate', 'sync_all']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    start_date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).optional(),
    end_date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).optional(),
    days: z.number().int().min(1).max(365).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    const hasRange = Boolean(value.start_date && value.end_date);
    const hasWindow = Boolean(value.days);
    if (!hasRange && !hasWindow) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'HEALTHKIT_SYNC_ALIAS_RANGE_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('healthkit-sync'),
    action: ActionSchema,
    alias_target: z.literal('healthkit-sync-apple'),
    deprecated_alias: z.literal(true),
    forwarded: z.boolean(),
    summary: z.string().min(10).max(4096)
  })
  .strict();
