import { z } from 'zod';

const ActionSchema = z.enum(['generate_video', 'check_status']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    prompt: z.string().min(2).max(1000).optional(),
    job_id: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'generate_video' && !value.prompt) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'VEO_PROMPT_REQUIRED' });
    }
    if (value.action === 'check_status' && !value.job_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'VEO_JOB_ID_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('veo'),
    action: ActionSchema,
    job_id: z.string().min(2).max(120),
    status: z.enum(['queued', 'processing', 'completed']),
    video_url: z.string().url().optional(),
    summary: z.string().min(10).max(4096)
  })
  .strict();
