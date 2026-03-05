import { z } from 'zod';

const ActionSchema = z.enum(['summarize_note', 'extract_actions']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    note_text: z.string().min(20).max(20000).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.note_text) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'GRANOLA_NOTE_TEXT_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('granola'),
    action: ActionSchema,
    summary: z.string().min(10).max(4096),
    action_items: z.array(z.string().min(2).max(300)).min(1).max(20),
    decisions: z.array(z.string().min(2).max(300)).max(20)
  })
  .strict();
