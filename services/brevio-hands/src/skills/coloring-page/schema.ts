import { z } from 'zod';

const ActionSchema = z.enum(['generate_from_prompt', 'generate_from_image']);
const ComplexitySchema = z.enum(['easy', 'medium', 'advanced']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    prompt: z.string().min(2).max(500).optional(),
    image_url: z.string().url().optional(),
    complexity: ComplexitySchema.optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'generate_from_prompt' && !value.prompt) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'COLORING_PAGE_PROMPT_REQUIRED' });
    }
    if (value.action === 'generate_from_image' && !value.image_url) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'COLORING_PAGE_IMAGE_URL_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('coloring-page'),
    action: ActionSchema,
    output_url: z.string().url(),
    page_size: z.enum(['A4', 'Letter']),
    line_density: z.enum(['low', 'medium', 'high']),
    summary: z.string().min(10).max(4096)
  })
  .strict();
