import { z } from 'zod';

const ActionSchema = z.enum(['run_research']);
const DepthSchema = z.enum(['standard', 'deep']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    topic: z.string().min(2).max(500).optional(),
    depth: DepthSchema.optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.topic) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'GEMINI_DEEP_RESEARCH_TOPIC_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('gemini-deep-research'),
    action: ActionSchema,
    report_sections: z.array(z.string().min(2).max(300)).min(1).max(20),
    citations: z.array(z.string().url()).min(1).max(20),
    summary: z.string().min(10).max(4096)
  })
  .strict();
