import { z } from 'zod';

const ActionSchema = z.enum(['clarify_problem', 'challenge_assumptions', 'decision_matrix']);

const MatrixRowSchema = z.object({
  option: z.string().min(2).max(180),
  expected_upside: z.string().min(2).max(240),
  key_risk: z.string().min(2).max(240),
  confidence_score: z.number().int().min(1).max(10)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    topic: z.string().min(5).max(240).optional(),
    constraints: z.array(z.string().min(2).max(180)).max(20).optional(),
    options: z.array(z.string().min(2).max(180)).max(12).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.topic?.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'THINKING_PARTNER_TOPIC_REQUIRED'
      });
    }

    if (value.action === 'decision_matrix' && (value.options?.length ?? 0) < 2) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'THINKING_PARTNER_OPTIONS_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('thinking-partner'),
    action: ActionSchema,
    reframed_problem: z.string().min(10).max(4096),
    questions: z.array(z.string().min(2).max(240)).max(15),
    assumptions_to_test: z.array(z.string().min(2).max(240)).max(15),
    decision_matrix: z.array(MatrixRowSchema).max(12).optional()
  })
  .strict();
