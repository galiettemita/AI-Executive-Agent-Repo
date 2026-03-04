import { z } from 'zod';

const ActionSchema = z.enum(['evaluate_decision']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    decision: z.string().min(5).max(500).optional(),
    options: z.array(z.string().min(2).max(200)).min(2).max(10).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.decision || !value.options) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'PROS_CONS_DECISION_FIELDS_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('pros-cons'),
    action: ActionSchema,
    options: z.array(
      z
        .object({
          option: z.string().min(2).max(200),
          pros: z.array(z.string().min(2).max(300)).max(10),
          cons: z.array(z.string().min(2).max(300)).max(10),
          score: z.number().min(0).max(100)
        })
        .strict()
    ),
    recommendation: z.string().min(2).max(200),
    summary: z.string().min(10).max(4096)
  })
  .strict();
