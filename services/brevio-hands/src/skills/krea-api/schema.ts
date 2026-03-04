import { z } from 'zod';

const ActionSchema = z.enum(['generate_image', 'upscale_image', 'list_models']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    prompt: z.string().min(2).max(600).optional(),
    image_url: z.string().url().optional(),
    model: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'generate_image' && !value.prompt) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'KREA_API_PROMPT_REQUIRED' });
    }
    if (value.action === 'upscale_image' && !value.image_url) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'KREA_API_IMAGE_URL_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('krea-api'),
    action: ActionSchema,
    image_url: z.string().url(),
    model: z.string().min(2).max(120),
    quality_score: z.number().min(0).max(1),
    summary: z.string().min(10).max(4096)
  })
  .strict();
