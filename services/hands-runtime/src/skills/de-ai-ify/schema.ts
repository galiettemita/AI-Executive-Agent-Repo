import { z } from 'zod';

const ActionSchema = z.enum(['rewrite_text', 'tone_check']);
const ToneSchema = z.enum(['casual', 'professional', 'direct']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    text: z.string().min(10).max(8000).optional(),
    target_tone: ToneSchema.optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.text) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'DE_AI_IFY_TEXT_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('de-ai-ify'),
    action: ActionSchema,
    rewritten_text: z.string().min(10).max(8000),
    detected_ai_markers: z.array(z.string().min(2).max(160)).max(20),
    summary: z.string().min(10).max(4096)
  })
  .strict();
