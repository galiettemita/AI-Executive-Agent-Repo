import { z } from 'zod';

const ActionSchema = z.enum(['generate_image', 'generate_video', 'generate_audio']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    prompt: z.string().min(2).max(600).optional(),
    model: z.string().min(2).max(120).optional(),
    size: z.string().min(2).max(80).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.prompt) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'POLLINATIONS_PROMPT_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('pollinations'),
    action: ActionSchema,
    asset_url: z.string().url(),
    model_used: z.string().min(2).max(120),
    summary: z.string().min(10).max(4096)
  })
  .strict();
