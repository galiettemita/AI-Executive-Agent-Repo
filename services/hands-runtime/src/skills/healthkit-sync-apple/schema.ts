import { z } from 'zod';

const ActionSchema = z.enum(['sync_steps', 'sync_sleep', 'sync_heart_rate', 'sync_all']);

const SnapshotSchema = z.object({
  metric: z.enum(['steps', 'sleep_hours', 'heart_rate_bpm']),
  value: z.number(),
  recorded_at: z.string().datetime()
});

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
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'HEALTHKIT_SYNC_APPLE_RANGE_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('healthkit-sync-apple'),
    action: ActionSchema,
    snapshots: z.array(SnapshotSchema).max(365),
    synced_metric_count: z.number().int().min(0),
    summary: z.string().min(10).max(4096)
  })
  .strict();
