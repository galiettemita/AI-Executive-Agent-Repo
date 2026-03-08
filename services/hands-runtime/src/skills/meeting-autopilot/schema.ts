import { z } from 'zod';

const ActionSchema = z.enum(['summarize_meeting', 'extract_actions', 'draft_follow_up']);

const ActionItemSchema = z.object({
  owner: z.string().min(2).max(120),
  task: z.string().min(2).max(240),
  due_hint: z.string().min(2).max(80).optional()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    meeting_title: z.string().min(2).max(180).optional(),
    transcript: z.string().min(20).max(16000).optional(),
    participants: z.array(z.string().min(2).max(120)).max(30).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.transcript?.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'MEETING_AUTOPILOT_TRANSCRIPT_REQUIRED'
      });
    }

    if (value.action === 'draft_follow_up' && !value.participants?.length) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'MEETING_AUTOPILOT_PARTICIPANTS_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('meeting-autopilot'),
    action: ActionSchema,
    summary: z.string().min(10).max(4096),
    decisions: z.array(z.string().min(2).max(240)).max(20),
    action_items: z.array(ActionItemSchema).max(30),
    follow_up_email: z.string().min(10).max(4096).optional()
  })
  .strict();
