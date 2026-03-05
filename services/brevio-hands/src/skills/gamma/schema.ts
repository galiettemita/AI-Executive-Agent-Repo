import { z } from 'zod';

const ActionSchema = z.enum(['create_deck', 'update_deck', 'export_deck']);
const FormatSchema = z.enum(['pdf', 'pptx']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    topic: z.string().min(2).max(240).optional(),
    deck_id: z.string().min(3).max(120).optional(),
    slide_count: z.number().int().min(1).max(80).optional(),
    format: FormatSchema.optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'create_deck' && !value.topic) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'GAMMA_TOPIC_REQUIRED' });
    }
    if ((value.action === 'update_deck' || value.action === 'export_deck') && !value.deck_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'GAMMA_DECK_ID_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('gamma'),
    action: ActionSchema,
    deck_id: z.string().min(3).max(120),
    title: z.string().min(2).max(240),
    slide_count: z.number().int().min(1).max(80),
    export_url: z.string().url().optional(),
    summary: z.string().min(10).max(4096)
  })
  .strict();
