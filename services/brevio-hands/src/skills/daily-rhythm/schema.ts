import { z } from 'zod';

const ActionSchema = z.enum(['compose_briefing', 'wind_down_prompt']);

const TaskSchema = z.object({
  title: z.string().min(2).max(180),
  due_time_local: z.string().regex(/^\d{2}:\d{2}$/).optional(),
  priority: z.enum(['high', 'medium', 'low']),
  estimated_minutes: z.number().int().min(5).max(600)
});

const MeetingSchema = z.object({
  title: z.string().min(2).max(180),
  start_local: z.string().regex(/^\d{2}:\d{2}$/),
  end_local: z.string().regex(/^\d{2}:\d{2}$/)
});

const ScheduleBlockSchema = z.object({
  title: z.string().min(2).max(180),
  start_local: z.string().regex(/^\d{2}:\d{2}$/),
  end_local: z.string().regex(/^\d{2}:\d{2}$/),
  kind: z.enum(['focus', 'meeting', 'admin', 'recovery'])
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    timezone: z.string().min(3).max(80),
    date: z.string().regex(/^\d{4}-\d{2}-\d{2}$/),
    wake_time_local: z.string().regex(/^\d{2}:\d{2}$/).optional(),
    tasks: z.array(TaskSchema).max(50).optional(),
    meetings: z.array(MeetingSchema).max(20).optional(),
    weather_summary: z.string().min(2).max(240).optional(),
    energy_level: z.enum(['low', 'steady', 'high']).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'compose_briefing' && !value.wake_time_local) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'DAILY_RHYTHM_WAKE_TIME_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('daily-rhythm'),
    action: ActionSchema,
    briefing_text: z.string().min(10).max(4096),
    priorities: z.array(z.string().min(2).max(180)).max(10),
    schedule_blocks: z.array(ScheduleBlockSchema).max(20),
    nudges: z.array(z.string().min(2).max(240)).max(10)
  })
  .strict();
