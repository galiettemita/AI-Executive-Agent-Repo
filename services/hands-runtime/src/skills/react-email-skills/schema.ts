import { z } from 'zod';

const ActionSchema = z.enum(['render_template', 'preview_message']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    template_id: z.string().min(2).max(120).optional(),
    subject: z.string().min(2).max(240).optional(),
    variables: z.record(z.string()).optional(),
    preview_to: z.string().email().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.template_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'REACT_EMAIL_TEMPLATE_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('react-email-skills'),
    action: ActionSchema,
    html: z.string().min(10).max(50000),
    text: z.string().min(10).max(50000),
    preview_id: z.string().min(2).max(120),
    summary: z.string().min(10).max(4096)
  })
  .strict();
