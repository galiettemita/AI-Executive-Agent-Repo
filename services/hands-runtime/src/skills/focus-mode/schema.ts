import { z } from 'zod';

const ActionSchema = z.enum(['start_session', 'check_in', 'end_session']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    goal: z.string().min(5).max(240).optional(),
    duration_minutes: z.number().int().min(15).max(240).optional(),
    session_id: z.string().min(6).max(80).optional(),
    distraction_note: z.string().min(2).max(240).optional(),
    completed_tasks: z.array(z.string().min(2).max(180)).max(20).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'start_session' && (!value.goal || !value.duration_minutes)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'FOCUS_MODE_START_FIELDS_REQUIRED'
      });
    }

    if ((value.action === 'check_in' || value.action === 'end_session') && !value.session_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'FOCUS_MODE_SESSION_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('focus-mode'),
    action: ActionSchema,
    session_id: z.string().min(6).max(80),
    status: z.enum(['active', 'checking_in', 'completed']),
    check_in_schedule: z.array(z.string().regex(/^\d{2}:\d{2}$/)).max(12),
    next_prompt: z.string().min(5).max(240),
    summary: z.string().min(5).max(4096).optional()
  })
  .strict();
