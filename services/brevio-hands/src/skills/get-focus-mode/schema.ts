import { z } from 'zod';

const ActionSchema = z.enum(['current_mode', 'upcoming_schedule']);

const ScheduleWindowSchema = z.object({
  starts_local: z.string().regex(/^\d{2}:\d{2}$/),
  ends_local: z.string().regex(/^\d{2}:\d{2}$/),
  mode: z.string().min(2).max(80)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    timezone: z.string().min(3).max(80).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('get-focus-mode'),
    action: ActionSchema,
    current_mode: z.string().min(2).max(80),
    schedule: z.array(ScheduleWindowSchema).max(20),
    summary: z.string().min(10).max(4096)
  })
  .strict();
