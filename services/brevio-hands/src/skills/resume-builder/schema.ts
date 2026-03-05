import { z } from 'zod';

const ActionSchema = z.enum(['generate', 'tailor', 'score']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    role: z.string().min(2).max(200).optional(),
    experience_bullets: z.array(z.string().min(2).max(300)).max(30).optional(),
    job_description: z.string().min(20).max(5000).optional(),
    resume_markdown: z.string().min(20).max(20000).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if ((value.action === 'generate' || value.action === 'tailor') && !value.role) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'RESUME_BUILDER_ROLE_REQUIRED' });
    }

    if (value.action === 'tailor' && !value.job_description) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'RESUME_BUILDER_JOB_DESCRIPTION_REQUIRED'
      });
    }

    if (value.action === 'score' && !value.resume_markdown) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'RESUME_BUILDER_RESUME_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('resume-builder'),
    action: ActionSchema,
    resume_markdown: z.string().optional(),
    score: z.number().min(0).max(100).optional(),
    recommendations: z.array(z.string()).optional()
  })
  .strict();
